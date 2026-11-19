// Package incus は、手元の Incus のシステムコンテナでチーム環境を作る。
//
// チーム環境のサーバーは、問題バンドルのテンプレートにあるサーバーごとに 1 台のコンテナとして作る。
// ポータルは状態を DB に持たず、読むたびにコンテナと、コンテナの中の印のファイルから組み立てる。
package incus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	incusclient "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/units"
	"golang.org/x/sync/errgroup"

	"github.com/traPtitech/piscon-portal-v2/server/domain"
	"github.com/traPtitech/piscon-portal-v2/server/services/instance"
)

var (
	_ instance.EnvironmentManager = (*Client)(nil)
	_ instance.Manager            = (*Client)(nil)
)

// コンテナに付ける設定値。user-data はゲスト API でこれらを読む。
const (
	keyEnvironment   = "user.piscon.environment"
	keyServer        = "user.piscon.server"
	keyTeamID        = "user.piscon.team-id"
	keySSHPublicKeys = "user.piscon.ssh-public-keys"
	// keyDeleting は、削除を依頼した時刻（RFC 3339）である。ゲストには関係しない。
	keyDeleting = "user.piscon.deleting"
)

// user-data が準備の結果を知らせる印のファイル。
const (
	readyMarker  = "/var/lib/piscon/ready"
	failedMarker = "/var/lib/piscon/failed"
	// maxFailureReasonBytes は、失敗の印から読む理由の長さの上限である。
	maxFailureReasonBytes = 1024
)

const (
	// maxContainerNameLen は、Incus のインスタンスの名前の長さの上限である。
	maxContainerNameLen = 63
	// environmentNameLen は、ユースケースが付けるチーム環境の名前（piscon-<UUID>）の長さである。
	environmentNameLen = len("piscon-") + 36
	// maxServerNameLen は、コンテナの名前（<チーム環境の名前>-<サーバーの名前>）が上限に収まる、サーバーの名前の長さである。
	maxServerNameLen = maxContainerNameLen - environmentNameLen - 1
)

// serverNamePattern は、コンテナの名前に使えるサーバーの名前である。CloudFormation の論理 ID は英数字だけからなる。
var serverNamePattern = regexp.MustCompile(`^[A-Za-z0-9]+$`)

// Config は、すべてのチーム環境に共通する、コンテナの作り方である。
type Config struct {
	// SocketPath は、Incus の API の Unix ソケットのパスである。
	SocketPath string
	// Servers は、チーム環境のサーバーの名前である。テンプレートの論理 ID をそのまま使う。
	Servers []string
	// Image は、コンテナを作るイメージのエイリアスである。
	Image    string
	UserData string
	CPU      int
	// Memory は、Incus の limits.memory に渡す値（4GiB など）である。
	Memory string
}

// Client は、Incus のコンテナでチーム環境とサーバーを扱う。
// プロバイダーがチーム環境を識別する値はチーム環境の名前、サーバーを識別する値はコンテナの名前である。
type Client struct {
	client incusclient.InstanceServer
	cfg    Config
	now    func() time.Time
}

// New は、設定を確かめてから Incus に接続する。
// サーバーの名前がコンテナの名前に使えない場合、メモリの量が読めない場合、
// Incus に接続できない場合、イメージのエイリアスがない場合はエラーを返す。
func New(ctx context.Context, cfg Config) (*Client, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	client, err := incusclient.ConnectIncusUnixWithContext(ctx, cfg.SocketPath, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to incus at %s: %w", cfg.SocketPath, err)
	}
	if _, _, err := client.GetImageAlias(cfg.Image); err != nil {
		return nil, fmt.Errorf("image alias %q: %w", cfg.Image, err)
	}
	return &Client{client: client, cfg: cfg, now: time.Now}, nil
}

func validateConfig(cfg Config) error {
	if len(cfg.Servers) == 0 {
		return errors.New("no servers")
	}
	for _, s := range cfg.Servers {
		if !serverNamePattern.MatchString(s) || len(s) > maxServerNameLen {
			return fmt.Errorf("server name %q must be at most %d letters or digits to fit in a container name", s, maxServerNameLen)
		}
	}
	if cfg.CPU <= 0 {
		return fmt.Errorf("cpu must be positive: %d", cfg.CPU)
	}
	if _, err := units.ParseByteSizeString(cfg.Memory); err != nil {
		return fmt.Errorf("memory %q: %w", cfg.Memory, err)
	}
	return nil
}

func containerName(environment, server string) string {
	return environment + "-" + server
}

// Create は、テンプレートの全サーバーのコンテナの作成と起動を依頼し、全コンテナが Incus に登録されるまで待つ。
// イメージの展開、起動、準備（user-data）の完了は待たない。失敗しても、作れたコンテナは消さない。
func (c *Client) Create(ctx context.Context, name string, params instance.EnvironmentParams) (string, error) {
	g, ctx := errgroup.WithContext(ctx)
	for _, server := range c.cfg.Servers {
		g.Go(func() error {
			op, err := c.client.CreateInstance(api.InstancesPost{
				Name:   containerName(name, server),
				Type:   api.InstanceTypeContainer,
				Source: api.InstanceSource{Type: "image", Alias: c.cfg.Image},
				Start:  true,
				InstancePut: api.InstancePut{Config: map[string]string{
					"limits.cpu":           fmt.Sprint(c.cfg.CPU),
					"limits.memory":        c.cfg.Memory,
					"cloud-init.user-data": c.cfg.UserData,
					keyEnvironment:         name,
					keyServer:              server,
					keyTeamID:              params.TeamID,
					keySSHPublicKeys:       strings.Join(params.SSHPublicKeys, "\n"),
				}},
			})
			if err != nil {
				return fmt.Errorf("create container for %s: %w", server, err)
			}
			if err := c.waitRegistered(ctx, containerName(name, server), op.Get().ID); err != nil {
				return fmt.Errorf("create container for %s: %w", server, err)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return "", fmt.Errorf("create environment %s: %w", name, err)
	}
	return name, nil
}

// registerTimeout は、作成を依頼してから、コンテナが Incus に登録されるまでを待つ時間である。
const registerTimeout = 30 * time.Second

// waitRegistered は、コンテナが Incus に登録されるまで待つ。作成の操作が先に失敗すれば、そのエラーを返す。
// 登録はすぐに済むが、イメージの展開はストレージによっては数十秒かかるので、作成の操作の完了は待たない。
// 登録の前に戻ると、直後の Get がコンテナを見つけられず、チーム環境を deleted とみなす。
func (c *Client) waitRegistered(ctx context.Context, container, operationID string) error {
	ctx, cancel := context.WithTimeout(ctx, registerTimeout)
	defer cancel()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		_, _, err := c.client.GetInstance(container)
		if err == nil {
			return nil
		}
		if !api.StatusErrorCheck(err, http.StatusNotFound) {
			return fmt.Errorf("get container %s: %w", container, err)
		}
		if op, _, err := c.client.GetOperation(operationID); err == nil && op.StatusCode.IsFinal() && op.Err != "" {
			return errors.New(op.Err)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for container %s to be registered: %w", container, ctx.Err())
		case <-ticker.C:
		}
	}
}

func (c *Client) Get(_ context.Context, name string) (domain.InfraEnvironment, error) {
	views := make([]serverView, 0, len(c.cfg.Servers))
	for _, server := range c.cfg.Servers {
		v, err := c.readServer(name, server)
		if err != nil {
			return domain.InfraEnvironment{}, err
		}
		views = append(views, v)
	}
	return environmentStatus(views, c.now()), nil
}

func (c *Client) readServer(environment, server string) (serverView, error) {
	v := serverView{Server: server, Container: containerName(environment, server)}
	inst, _, err := c.client.GetInstance(v.Container)
	if api.StatusErrorCheck(err, http.StatusNotFound) {
		return v, nil
	}
	if err != nil {
		return serverView{}, fmt.Errorf("get container %s: %w", v.Container, err)
	}
	v.Exists = true
	v.CreatedAt = inst.CreatedAt
	if s, ok := inst.Config[keyDeleting]; ok {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return serverView{}, fmt.Errorf("container %s: parse %s: %w", v.Container, keyDeleting, err)
		}
		v.DeletingSince = &t
		// 消えかけのコンテナのファイルは読めないことがあり、読んでも状態は変わらない。
		return v, nil
	}

	_, v.Ready, err = c.readFile(v.Container, readyMarker)
	if err != nil {
		return serverView{}, err
	}
	reason, failed, err := c.readFile(v.Container, failedMarker)
	if err != nil {
		return serverView{}, err
	}
	if failed {
		s := strings.TrimSpace(reason)
		v.FailureReason = &s
	}
	return v, nil
}

// readFile は、コンテナの中のファイルを先頭から上限まで読む。ファイルがなければ false を返す。
func (c *Client) readFile(container, path string) (string, bool, error) {
	r, _, err := c.client.GetInstanceFile(container, path)
	if api.StatusErrorCheck(err, http.StatusNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read %s in %s: %w", path, container, err)
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(r, maxFailureReasonBytes))
	if err != nil {
		return "", false, fmt.Errorf("read %s in %s: %w", path, container, err)
	}
	return string(data), true, nil
}

// Delete は、チーム環境のコンテナを ephemeral にしてから止める。止まったコンテナは Incus が消す。
// 止まっているコンテナは直接消す。どちらも完了は待たない。何度呼んでもよい。
func (c *Client) Delete(_ context.Context, name string) error {
	for _, server := range c.cfg.Servers {
		if err := c.deleteContainer(containerName(name, server)); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) deleteContainer(container string) error {
	inst, etag, err := c.client.GetInstance(container)
	if api.StatusErrorCheck(err, http.StatusNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get container %s: %w", container, err)
	}

	if _, ok := inst.Config[keyDeleting]; !ok || !inst.Ephemeral {
		put := inst.Writable()
		put.Ephemeral = true
		if !ok {
			put.Config[keyDeleting] = c.now().UTC().Format(time.RFC3339)
		}
		op, err := c.client.UpdateInstance(container, put, etag)
		if err != nil {
			return fmt.Errorf("mark container %s for deletion: %w", container, err)
		}
		if err := op.Wait(); err != nil {
			return fmt.Errorf("mark container %s for deletion: %w", container, err)
		}
	}

	// 稼働中のコンテナは削除できないので、ephemeral のまま止めて Incus に消させる。
	if inst.StatusCode == api.Stopped {
		if _, err := c.client.DeleteInstance(container); err != nil {
			return fmt.Errorf("delete container %s: %w", container, err)
		}
		return nil
	}
	if err := c.requestState(container, "stop"); err != nil {
		return err
	}
	return nil
}

func (c *Client) GetByIDs(_ context.Context, ids []string) ([]domain.InfraInstance, error) {
	instances := make([]domain.InfraInstance, 0, len(ids))
	for _, id := range ids {
		state, _, err := c.client.GetInstanceState(id)
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("get state of container %s: %w", id, err)
		}
		inst := domain.InfraInstance{ProviderInstanceID: id, Status: instanceStatus(state.StatusCode)}
		if ip, ok := ipv4(state, "eth0"); ok {
			// 手元では、Incus を動かすマシンからコンテナのアドレスに直接 SSH する。
			inst.PrivateIP = &ip
			inst.PublicIP = &ip
		}
		instances = append(instances, inst)
	}
	return instances, nil
}

func instanceStatus(code api.StatusCode) domain.InstanceStatus {
	switch code {
	case api.Running, api.Ready:
		return domain.InstanceStatusRunning
	case api.Stopped:
		return domain.InstanceStatusStopped
	default:
		return domain.InstanceStatusUnknown
	}
}

func ipv4(state *api.InstanceState, nic string) (string, bool) {
	for _, a := range state.Network[nic].Addresses {
		if a.Family == "inet" && a.Scope == "global" {
			return a.Address, true
		}
	}
	return "", false
}

// Stop は、コンテナの強制停止を依頼する。停止の完了は待たない。
func (c *Client) Stop(_ context.Context, inst domain.InfraInstance) error {
	return c.requestState(inst.ProviderInstanceID, "stop")
}

// Start は、コンテナの起動を依頼する。起動の完了は待たない。
func (c *Client) Start(_ context.Context, inst domain.InfraInstance) error {
	return c.requestState(inst.ProviderInstanceID, "start")
}

func (c *Client) requestState(container, action string) error {
	state := api.InstanceStatePut{Action: action}
	if action == "stop" {
		state.Force = true
		state.Timeout = -1
	}
	if _, err := c.client.UpdateInstanceState(container, state, ""); err != nil {
		return fmt.Errorf("%s container %s: %w", action, container, err)
	}
	return nil
}
