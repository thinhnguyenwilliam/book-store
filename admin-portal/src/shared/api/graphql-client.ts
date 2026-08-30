import { ApiError, apiRequest } from './http-client'

interface GraphQLError {
  message: string
  extensions?: { code?: string }
}

interface GraphQLResponse<T> {
  data?: T
  errors?: GraphQLError[]
}

export async function graphQLRequest<T>(
  query: string,
  variables: Record<string, unknown>,
  signal?: AbortSignal,
): Promise<T> {
  const response = await apiRequest<GraphQLResponse<T>>('/graphql', {
    method: 'POST',
    data: { query, variables },
    ...(signal ? { signal } : {}),
  })
  const firstError = response.errors?.[0]
  if (firstError) {
    throw new ApiError(200, firstError.message, undefined, firstError.extensions?.code)
  }
  if (!response.data) throw new ApiError(200, 'GraphQL response does not contain data.')
  return response.data
}
