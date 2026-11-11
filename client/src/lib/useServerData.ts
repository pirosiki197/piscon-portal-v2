import { api } from '@/api'
import router from '@/router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'

export const useUsersData = () =>
  useQuery({
    queryKey: ['users'],
    queryFn: () => api.GET('/users').then((r) => r.data),
  })

export const useMe = () =>
  useQuery({
    queryKey: ['me'],
    queryFn: () => api.GET('/users/me').then((r) => r.data),
  })

export const useTeamBenches = (teamId: string) =>
  useQuery({
    queryKey: ['team-benches', teamId],
    queryFn: () =>
      api.GET('/teams/{teamId}/benchmarks', { params: { path: { teamId } } }).then((r) => r.data),
  })

const fetchTeamBench = (teamId: string, benchmarkId: string) =>
  api
    .GET('/teams/{teamId}/benchmarks/{benchmarkId}', {
      params: { path: { teamId, benchmarkId } },
    })
    .then((r) => r.data)

export const useTeamBench = (
  teamId: string,
  benchmarkId: string,
  options?: {
    // ポーリング間隔(ms)を返す。false を返すとポーリングを止める
    refetchInterval?: (bench: Awaited<ReturnType<typeof fetchTeamBench>>) => number | false
  },
) =>
  useQuery({
    queryKey: ['team-bench', teamId, benchmarkId],
    queryFn: () => fetchTeamBench(teamId, benchmarkId),
    refetchInterval: (query) => options?.refetchInterval?.(query.state.data) ?? false,
  })

export const useTeamInstances = (teamId: string) =>
  useQuery({
    queryKey: ['team-instances', teamId],
    queryFn: () =>
      api.GET('/teams/{teamId}/instances', { params: { path: { teamId } } }).then((r) => r.data),
  })

export const useTeam = (teamId: string) =>
  useQuery({
    queryKey: ['team', teamId],
    queryFn: () => api.GET('/teams/{teamId}', { params: { path: { teamId } } }).then((r) => r.data),
  })

// API のエラーレスポンスを Error にする。message があればそれを使う。
const toError = (error: unknown) =>
  new Error((error as { message?: string } | undefined)?.message ?? '操作に失敗しました')

// チーム環境がなければ null になる。
export const useTeamEnvironment = (teamId: string) =>
  useQuery({
    queryKey: ['team-environment', teamId],
    queryFn: () =>
      api.GET('/teams/{teamId}/environment', { params: { path: { teamId } } }).then((r) => {
        if (r.response.status === 404) return null
        if (r.error !== undefined) throw toError(r.error)
        return r.data ?? null
      }),
  })

const useTeamEnvironmentMutation = (
  mutationFn: (teamId: string) => Promise<{ error?: unknown }>,
) => {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async (params: { teamId: string }) => {
      const { error } = await mutationFn(params.teamId)
      if (error !== undefined) throw toError(error)
    },
    onSettled: (_, __, params) => {
      void client.invalidateQueries({ queryKey: ['team-environment', params.teamId] })
      void client.invalidateQueries({ queryKey: ['team-instances', params.teamId] })
      void client.invalidateQueries({ queryKey: ['instances'] })
    },
  })
}

export const useCreateTeamEnvironment = () =>
  useTeamEnvironmentMutation((teamId) =>
    api.POST('/teams/{teamId}/environment', { params: { path: { teamId } } }),
  )

export const useDeleteTeamEnvironment = () =>
  useTeamEnvironmentMutation((teamId) =>
    api.DELETE('/teams/{teamId}/environment', { params: { path: { teamId } } }),
  )

export const useStartTeamEnvironment = () =>
  useTeamEnvironmentMutation((teamId) =>
    api.PATCH('/teams/{teamId}/environment', {
      params: { path: { teamId } },
      body: { operation: 'start' },
    }),
  )

export const useStopTeamEnvironment = () =>
  useTeamEnvironmentMutation((teamId) =>
    api.PATCH('/teams/{teamId}/environment', {
      params: { path: { teamId } },
      body: { operation: 'stop' },
    }),
  )

export const useCreateTeam = () => {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (params: { name: string; members: string[] }) =>
      api.POST('/teams', {
        body: params,
      }),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ['teams'] })
      void client.invalidateQueries({ queryKey: ['me'] })
    },
  })
}

export const useUpdateTeam = () => {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (params: {
      teamId: string
      name?: string
      members?: string[]
      githubIds?: string[]
    }) =>
      api.PATCH('/teams/{teamId}', {
        params: { path: { teamId: params.teamId } },
        body: { name: params.name, members: params.members, githubIds: params.githubIds },
      }),
    onSuccess: (_, params) => {
      void client.invalidateQueries({ queryKey: ['team', params.teamId] })
      void client.invalidateQueries({ queryKey: ['teams'] })
      void client.invalidateQueries({ queryKey: ['me'] })
    },
  })
}

export const useEnqueueBenchmark = (options?: { redirect?: boolean }) => {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (params: { teamId: string; instanceId: string }) =>
      api.POST('/benchmarks', {
        body: { instanceId: params.instanceId },
      }),
    onSuccess: (res, params) => {
      void client.invalidateQueries({ queryKey: ['team-benches', params.teamId] })
      void client.invalidateQueries({ queryKey: ['benches'] })
      if (options?.redirect && res.data !== undefined) {
        void router.push(`/benches/${res.data.id}`)
      }
    },
  })
}

export const useAllInstances = () =>
  useQuery({
    queryKey: ['instances'],
    queryFn: () => api.GET('/instances').then((r) => r.data),
  })

export const useTeamsData = () =>
  useQuery({
    queryKey: ['teams'],
    queryFn: () => api.GET('/teams').then((r) => r.data),
  })

export const useAllBenches = () =>
  useQuery({
    queryKey: ['benches'],
    queryFn: () => api.GET('/benchmarks').then((r) => r.data),
  })

export const useBench = (benchmarkId: string) =>
  useQuery({
    queryKey: ['bench', benchmarkId],
    queryFn: () =>
      api
        .GET('/benchmarks/{benchmarkId}', { params: { path: { benchmarkId } } })
        .then((r) => r.data),
  })

export const useDocs = () =>
  useQuery({
    queryKey: ['docs'],
    queryFn: () => api.GET('/docs').then((r) => r.data),
  })

export const useUpdateDocs = () => {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (params: { body: string }) =>
      api.PATCH('/docs', {
        body: params,
      }),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ['docs'] })
    },
  })
}

export const useUpdateAdmins = () => {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (params: string[]) =>
      api.PUT('/admins', {
        body: params,
      }),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ['admins'] })
      void client.invalidateQueries({ queryKey: ['users'] })
    },
  })
}

export const useScores = () =>
  useQuery({
    queryKey: ['scores'],
    queryFn: () => api.GET('/scores').then((r) => r.data),
  })

export const useRanking = (orderBy: 'latest' | 'highest') =>
  useQuery({
    queryKey: ['ranking', orderBy],
    queryFn: () =>
      api.GET('/scores/ranking', { params: { query: { orderBy } } }).then((r) => r.data),
  })

export const useBenchmarkQueue = () =>
  useQuery({
    queryKey: ['benchmark-queue'],
    queryFn: () => api.GET('/benchmarks/queue').then((r) => r.data),
  })
