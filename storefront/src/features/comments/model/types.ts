export interface Comment {
  id: string
  book_id: string
  author_id: string
  author_name: string
  parent_id?: string
  root_id: string
  depth: number
  content: string
  status: 'published' | 'hidden' | 'deleted'
  reply_count: number
  created_at: string
  updated_at: string
}

export interface CommentPage {
  data: Comment[]
  pagination: { next_cursor?: string; has_more: boolean }
}
