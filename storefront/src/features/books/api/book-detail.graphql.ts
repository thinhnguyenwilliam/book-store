import { graphQLRequest } from '@/shared/api/graphql-client'
import type { Comment } from '@/features/comments/model/types'
import type { Book } from '../model/types'

interface GraphQLBook {
  id: string
  title: string
  author: string
  isbn: string
  priceCents: number
  stock: number
  createdAt: string
  updatedAt: string
}

interface GraphQLComment {
  id: string
  bookId: string
  authorId: string
  authorName: string
  parentId?: string
  rootId: string
  depth: number
  content: string
  status: Comment['status']
  replyCount: number
  createdAt: string
  updatedAt: string
}

interface BookDetailResult {
  bookDetail: {
    book: GraphQLBook
    comments: { nodes: GraphQLComment[] }
  }
}

const BOOK_DETAIL_QUERY = `
  query BookDetail($id: ID!, $commentsFirst: Int!) {
    bookDetail(id: $id, commentsFirst: $commentsFirst) {
      book {
        id title author isbn priceCents stock createdAt updatedAt
      }
      comments {
        nodes {
          id bookId authorId authorName parentId rootId depth content status replyCount createdAt updatedAt
        }
        pageInfo { hasNextPage endCursor }
      }
    }
  }
`

export async function getBookDetail(
  id: string,
  signal?: AbortSignal,
): Promise<{
  book: Book
  comments: Comment[]
}> {
  const { bookDetail } = await graphQLRequest<BookDetailResult>(
    BOOK_DETAIL_QUERY,
    { id, commentsFirst: 20 },
    signal,
  )
  return {
    book: {
      id: bookDetail.book.id,
      title: bookDetail.book.title,
      author: bookDetail.book.author,
      isbn: bookDetail.book.isbn,
      price_cents: bookDetail.book.priceCents,
      stock: bookDetail.book.stock,
      created_at: bookDetail.book.createdAt,
      updated_at: bookDetail.book.updatedAt,
    },
    comments: bookDetail.comments.nodes.map((item) => ({
      id: item.id,
      book_id: item.bookId,
      author_id: item.authorId,
      author_name: item.authorName,
      ...(item.parentId ? { parent_id: item.parentId } : {}),
      root_id: item.rootId,
      depth: item.depth,
      content: item.content,
      status: item.status,
      reply_count: item.replyCount,
      created_at: item.createdAt,
      updated_at: item.updatedAt,
    })),
  }
}
