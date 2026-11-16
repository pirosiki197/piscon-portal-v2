package bundle

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testTemplate は、短縮形のタグを含む CloudFormation テンプレートである。
const testTemplate = `
Parameters:
  ImageId:
    Type: AWS::EC2::Image::Id
Resources:
  Webapp:
    Type: AWS::EC2::Instance
    Properties:
      ImageId: !Ref ImageId
  Router:
    Type: AWS::EC2::Instance
    Properties:
      ImageId: !Ref ImageId
      UserData: !Base64
        Fn::Sub: echo ${Webapp.PrivateIp}
  Eip:
    Type: AWS::EC2::EIP
`

// newBundleDir は、bench ディレクトリ、manual.md、template.yaml を含むバンドルを作り、manifest を書き込む。
func newBundleDir(t *testing.T, manifest string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "bench"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manual.md"), []byte("# manual"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "user-data.sh"), []byte("#!/bin/sh\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "template.yaml"), []byte(testTemplate), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "empty.yaml"), []byte("Resources:\n  Eip:\n    Type: AWS::EC2::EIP\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ManifestFile), []byte(manifest), 0o644))
	return dir
}

func TestLoadCommand(t *testing.T) {
	dir := newBundleDir(t, `
name: sample
environment:
  template: template.yaml
  imageId: ami-0123
bench:
  target: Router
  kind: command
  dir: bench
  command: ["./bench", "--target", "{{ .TargetIP }}"]
  timeout: 2m
manual: manual.md
`)

	b, err := Load(dir)
	require.NoError(t, err)

	assert.Equal(t, Bundle{
		Name: "sample",
		Environment: Environment{
			TemplatePath: filepath.Join(dir, "template.yaml"),
			Servers:      []string{"Router", "Webapp"},
			ImageID:      "ami-0123",
		},
		Bench: Bench{Target: "Router", Command: &CommandBench{
			Dir:     filepath.Join(dir, "bench"),
			Command: []string{"./bench", "--target", "{{ .TargetIP }}"},
			Timeout: 2 * time.Minute,
		}},
		ManualPath: filepath.Join(dir, "manual.md"),
	}, b)
}

func TestLoadAdapter(t *testing.T) {
	dir := newBundleDir(t, `
name: isucon13
environment:
  template: template.yaml
bench:
  kind: adapter
  adapter: isucon13
  options:
    benchmarker-path: /home/isucon/bench
    dns-port: 53
    enable-ssl: true
manual: manual.md
`)

	b, err := Load(dir)
	require.NoError(t, err)

	assert.Empty(t, b.Bench.Target)
	assert.Nil(t, b.Bench.Command)
	assert.Equal(t, &AdapterBench{
		Adapter: "isucon13",
		Options: map[string]any{"benchmarker-path": "/home/isucon/bench", "dns-port": 53, "enable-ssl": true},
	}, b.Bench.Adapter)
}

func TestLoadIncus(t *testing.T) {
	dir := newBundleDir(t, `
name: private-isu
environment:
  template: template.yaml
  incus:
    image: private-isu
    userData: user-data.sh
    limits:
      cpu: 2
      memory: 4GiB
bench:
  kind: adapter
  adapter: private-isu
manual: manual.md
`)

	b, err := Load(dir)
	require.NoError(t, err)

	assert.Equal(t, &IncusEnvironment{
		Image:        "private-isu",
		UserDataPath: filepath.Join(dir, "user-data.sh"),
		CPU:          2,
		Memory:       "4GiB",
	}, b.Environment.Incus)
}

func TestLoadRejectsInvalidIncus(t *testing.T) {
	const head = "name: x\nmanual: manual.md\nbench:\n  kind: adapter\n  adapter: a\nenvironment:\n  template: template.yaml\n  incus:\n"
	tests := map[string]string{
		"missing image":     "    userData: user-data.sh\n    limits: {cpu: 2, memory: 4GiB}\n",
		"missing userData":  "    image: i\n    limits: {cpu: 2, memory: 4GiB}\n",
		"userData outside":  "    image: i\n    userData: ../user-data.sh\n    limits: {cpu: 2, memory: 4GiB}\n",
		"missing limits":    "    image: i\n    userData: user-data.sh\n",
		"zero cpu":          "    image: i\n    userData: user-data.sh\n    limits: {cpu: 0, memory: 4GiB}\n",
		"missing memory":    "    image: i\n    userData: user-data.sh\n    limits: {cpu: 2}\n",
		"unknown limit key": "    image: i\n    userData: user-data.sh\n    limits: {cpu: 2, memory: 4GiB, disk: 10GiB}\n",
	}

	for name, incus := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Load(newBundleDir(t, head+incus))
			assert.Error(t, err)
		})
	}
}

func TestLoadRejectsInvalidManifest(t *testing.T) {
	const commandBench = `
environment:
  template: template.yaml
bench:
  kind: command
  dir: bench
  command: ["./bench"]
`
	tests := map[string]string{
		"missing name":            commandBench + "manual: manual.md\n",
		"unknown key":             "name: x\nmanual: manual.md\nunknown: 1\n" + commandBench,
		"missing kind":            "name: x\nmanual: manual.md\nbench:\n  dir: bench\n",
		"unknown kind":            "name: x\nmanual: manual.md\nbench:\n  kind: shell\n",
		"missing command":         "name: x\nmanual: manual.md\nbench:\n  kind: command\n  dir: bench\n",
		"bench dir outside":       "name: x\nmanual: manual.md\nbench:\n  kind: command\n  dir: ../bench\n  command: [a]\n",
		"bench dir not found":     "name: x\nmanual: manual.md\nbench:\n  kind: command\n  dir: nope\n  command: [a]\n",
		"bench dir is a file":     "name: x\nmanual: manual.md\nbench:\n  kind: command\n  dir: manual.md\n  command: [a]\n",
		"invalid timeout":         "name: x\nmanual: manual.md\nbench:\n  kind: command\n  dir: bench\n  command: [a]\n  timeout: soon\n",
		"command with adapter":    "name: x\nmanual: manual.md\nbench:\n  kind: command\n  dir: bench\n  command: [a]\n  adapter: isucon13\n",
		"adapter without name":    "name: x\nmanual: manual.md\nbench:\n  kind: adapter\n",
		"adapter with command":    "name: x\nmanual: manual.md\nbench:\n  kind: adapter\n  adapter: isucon13\n  command: [a]\n",
		"missing manual":          "name: x\n" + commandBench,
		"manual not found":        "name: x\nmanual: nope.md\n" + commandBench,
		"manual outside (abs)":    "name: x\nmanual: /etc/passwd\n" + commandBench,
		"manual is the bench dir": "name: x\nmanual: bench\n" + commandBench,
		"missing template":        "name: x\nmanual: manual.md\nbench:\n  kind: command\n  dir: bench\n  command: [a]\n",
		"template without server": "name: x\nmanual: manual.md\nenvironment:\n  template: empty.yaml\nbench:\n  kind: command\n  dir: bench\n  command: [a]\n",
		"target is not a server":  "name: x\nmanual: manual.md\nenvironment:\n  template: template.yaml\nbench:\n  target: Eip\n  kind: command\n  dir: bench\n  command: [a]\n",
	}

	for name, manifest := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Load(newBundleDir(t, manifest))
			assert.Error(t, err)
		})
	}
}

func TestLoadMissingManifest(t *testing.T) {
	_, err := Load(t.TempDir())
	assert.ErrorIs(t, err, os.ErrNotExist)
}
