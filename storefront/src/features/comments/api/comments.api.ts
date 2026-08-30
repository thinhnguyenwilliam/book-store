import { apiRequest } from '@/shared/api/http-client'

import type { Comment, CommentPage } from '../model/types'

export function listBookComments(bookId: string, cursor?: string): Promise<CommentPage> {
  return apiRequest(`/api/v1/books/${encodeURIComponent(bookId)}/comments`, {
    params: cursor ? { limit: 20, cursor } : { limit: 20 },
  })
}

export function listReplies(rootId: string, cursor?: string): Promise<CommentPage> {
  return apiRequest(`/api/v1/comments/${encodeURIComponent(rootId)}/replies`, {
    params: cursor ? { limit: 50, cursor } : { limit: 50 },
  })
}

export function createComment(
  bookId: string,
  payload: { content: string; parent_id?: string },
): Promise<Comment> {
  return apiRequest(`/api/v1/books/${encodeURIComponent(bookId)}/comments`, {
    method: 'POST',
    data: payload,
  })
}

export function updateComment(id: string, content: string): Promise<Comment> {
  return apiRequest(`/api/v1/comments/${encodeURIComponent(id)}`, {
    method: 'PUT',
    data: { content },
  })
}

export function deleteComment(id: string): Promise<Comment> {
  return apiRequest(`/api/v1/comments/${encodeURIComponent(id)}`, { method: 'DELETE' })
}
