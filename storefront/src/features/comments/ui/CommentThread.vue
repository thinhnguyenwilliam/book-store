<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useAuthStore } from '@/features/auth/model/auth.store'
import { useNotificationStore } from '@/features/notifications/model/notification.store'
import { initials } from '@/shared/lib/format'
import * as commentsApi from '../api/comments.api'
import type { Comment } from '../model/types'

const props = defineProps<{ bookId: string; initialComments?: Comment[] }>()
const auth = useAuthStore()
const notifications = useNotificationStore()
const route = useRoute()
const router = useRouter()
const roots = ref<Comment[]>([])
const replies = ref<Record<string, Comment[]>>({})
const loadedThreads = ref<Record<string, boolean>>({})
const loading = ref(true)
const submitting = ref(false)
const content = ref('')
const replyTo = ref<Comment>()
const replyContent = ref('')
const error = ref('')
const remaining = computed(() => 2000 - Array.from(content.value).length)

onMounted(() => {
  if (props.initialComments) {
    roots.value = props.initialComments
    loading.value = false
    return
  }
  void loadRoots()
})

async function loadRoots(): Promise<void> {
  loading.value = true
  try {
    roots.value = (await commentsApi.listBookComments(props.bookId)).data
  } catch {
    error.value = 'Chưa thể tải bình luận.'
  } finally {
    loading.value = false
  }
}

async function loadReplies(root: Comment): Promise<void> {
  if (loadedThreads.value[root.id]) return
  try {
    replies.value[root.id] = (await commentsApi.listReplies(root.id)).data
    loadedThreads.value[root.id] = true
  } catch {
    notifications.show('Chưa thể tải các câu trả lời.', 'error')
  }
}

async function submitRoot(): Promise<void> {
  if (!auth.isAuthenticated) return goToLogin()
  if (!content.value.trim() || submitting.value) return
  submitting.value = true
  try {
    roots.value.unshift(await commentsApi.createComment(props.bookId, { content: content.value }))
    content.value = ''
    notifications.show('Bình luận đã được đăng.', 'success')
  } catch {
    notifications.show('Không thể đăng bình luận.', 'error')
  } finally {
    submitting.value = false
  }
}

function startReply(item: Comment): void {
  if (!auth.isAuthenticated) return goToLogin()
  replyTo.value = item
  replyContent.value = ''
}

async function submitReply(): Promise<void> {
  const parent = replyTo.value
  if (!parent || !replyContent.value.trim() || submitting.value) return
  submitting.value = true
  try {
    const created = await commentsApi.createComment(props.bookId, {
      content: replyContent.value,
      parent_id: parent.id,
    })
    const rootId = parent.root_id
    replies.value[rootId] = [...(replies.value[rootId] || []), created]
    loadedThreads.value[rootId] = true
    const root = roots.value.find((item) => item.id === rootId)
    if (root) root.reply_count += 1
    replyTo.value = undefined
    notifications.show('Câu trả lời đã được đăng.', 'success')
  } catch {
    notifications.show('Không thể đăng câu trả lời.', 'error')
  } finally {
    submitting.value = false
  }
}

async function remove(item: Comment): Promise<void> {
  if (!window.confirm('Bạn muốn xoá bình luận này?')) return
  try {
    const deleted = await commentsApi.deleteComment(item.id)
    Object.assign(item, deleted)
    notifications.show('Đã xoá bình luận.', 'success')
  } catch {
    notifications.show('Không thể xoá bình luận.', 'error')
  }
}

function goToLogin(): void {
  void router.push({ name: 'login', query: { redirect: route.fullPath } })
}

function canDelete(item: Comment): boolean {
  return item.status === 'published' && auth.profile?.id === item.author_id
}

function formatTime(value: string): string {
  return new Intl.DateTimeFormat('vi-VN', { dateStyle: 'medium', timeStyle: 'short' }).format(
    new Date(value),
  )
}
</script>

<template>
  <section class="comments" aria-labelledby="comments-title">
    <div class="comments__heading">
      <div>
        <p class="eyebrow">Góc độc giả</p>
        <h2 id="comments-title">Bình luận về cuốn sách</h2>
      </div>
      <span>{{ roots.length }} cuộc trò chuyện</span>
    </div>

    <form class="comment-form" @submit.prevent="submitRoot">
      <textarea
        v-model="content"
        maxlength="2000"
        rows="4"
        placeholder="Chia sẻ cảm nhận của bạn…"
      />
      <div>
        <small>{{ remaining }} ký tự</small
        ><button class="button button--primary" type="submit" :disabled="submitting">
          {{ auth.isAuthenticated ? 'Đăng bình luận' : 'Đăng nhập để bình luận' }}
        </button>
      </div>
    </form>

    <p v-if="loading" class="comments__state">Đang tải bình luận…</p>
    <p v-else-if="error" class="comments__state">{{ error }}</p>
    <p v-else-if="!roots.length" class="comments__state">Hãy là người đầu tiên chia sẻ cảm nhận.</p>

    <div v-else class="comment-list">
      <article v-for="root in roots" :key="root.id" class="comment-card">
        <div class="comment-avatar">{{ initials(root.author_name || '?') }}</div>
        <div class="comment-body">
          <header>
            <strong>{{ root.author_name || 'Độc giả' }}</strong
            ><time :datetime="root.created_at">{{ formatTime(root.created_at) }}</time>
          </header>
          <p :class="{ 'is-tombstone': root.status !== 'published' }">{{ root.content }}</p>
          <div class="comment-actions">
            <button
              v-if="root.depth < 3 && root.status === 'published'"
              type="button"
              @click="startReply(root)"
            >
              Trả lời
            </button>
            <button v-if="canDelete(root)" type="button" @click="remove(root)">Xoá</button>
            <button
              v-if="root.reply_count && !loadedThreads[root.id]"
              type="button"
              @click="loadReplies(root)"
            >
              Xem {{ root.reply_count }} câu trả lời
            </button>
          </div>

          <div v-if="loadedThreads[root.id]" class="reply-list">
            <article
              v-for="reply in replies[root.id]"
              :key="reply.id"
              class="reply-card"
              :style="{ '--reply-depth': Math.max(1, reply.depth) }"
            >
              <div class="comment-avatar comment-avatar--small">
                {{ initials(reply.author_name || '?') }}
              </div>
              <div class="comment-body">
                <header>
                  <strong>{{ reply.author_name || 'Độc giả' }}</strong
                  ><time :datetime="reply.created_at">{{ formatTime(reply.created_at) }}</time>
                </header>
                <p :class="{ 'is-tombstone': reply.status !== 'published' }">{{ reply.content }}</p>
                <div class="comment-actions">
                  <button
                    v-if="reply.depth < 3 && reply.status === 'published'"
                    type="button"
                    @click="startReply(reply)"
                  >
                    Trả lời</button
                  ><button v-if="canDelete(reply)" type="button" @click="remove(reply)">Xoá</button>
                </div>
              </div>
            </article>
          </div>
        </div>
      </article>
    </div>

    <div v-if="replyTo" class="reply-composer">
      <form @submit.prevent="submitReply">
        <header>
          <strong>Trả lời {{ replyTo.author_name || 'độc giả' }}</strong
          ><button type="button" @click="replyTo = undefined">Đóng</button>
        </header>
        <textarea v-model="replyContent" maxlength="2000" rows="3" autofocus /><button
          class="button button--primary"
          type="submit"
          :disabled="submitting"
        >
          Gửi trả lời
        </button>
      </form>
    </div>
  </section>
</template>

<style scoped>
.comments {
  margin-top: 100px;
  padding-top: 64px;
  border-top: 1px solid var(--color-line);
}
.comments__heading {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 20px;
}
.comments__heading h2 {
  margin: 10px 0 0;
  font-family: var(--font-display);
  font-size: clamp(2rem, 4vw, 3.4rem);
  font-weight: 550;
}
.comments__heading > span,
.comments__state {
  color: var(--color-muted);
}
.comment-form {
  display: grid;
  gap: 12px;
  margin: 34px 0 46px;
  padding: 20px;
  border: 1px solid var(--color-line);
  border-radius: 18px;
  background: white;
}
.comment-form textarea,
.reply-composer textarea {
  width: 100%;
  resize: vertical;
  border: 0;
  outline: 0;
  color: var(--color-ink);
  background: transparent;
  font: inherit;
  line-height: 1.6;
}
.comment-form > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.comment-form small {
  color: var(--color-muted);
}
.comment-list {
  display: grid;
  gap: 16px;
}
.comment-card,
.reply-card {
  display: grid;
  grid-template-columns: 42px 1fr;
  gap: 14px;
  padding: 22px;
  border: 1px solid var(--color-line);
  border-radius: 18px;
  background: white;
}
.comment-avatar {
  display: grid;
  width: 42px;
  height: 42px;
  place-items: center;
  border-radius: 50%;
  color: white;
  background: var(--color-brand);
  font-size: 0.72rem;
  font-weight: 800;
}
.comment-avatar--small {
  width: 34px;
  height: 34px;
}
.comment-body header {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: baseline;
}
.comment-body header time {
  color: var(--color-muted);
  font-size: 0.72rem;
}
.comment-body > p {
  margin: 9px 0;
  line-height: 1.65;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.is-tombstone {
  color: var(--color-muted);
  font-style: italic;
}
.comment-actions {
  display: flex;
  gap: 15px;
}
.comment-actions button,
.reply-composer header button {
  padding: 0;
  border: 0;
  color: var(--color-brand);
  background: none;
  font-size: 0.75rem;
  font-weight: 750;
  cursor: pointer;
}
.reply-list {
  display: grid;
  gap: 10px;
  margin-top: 18px;
}
.reply-card {
  margin-left: calc((var(--reply-depth) - 1) * 20px);
  padding: 15px;
  border-radius: 14px;
  background: var(--color-paper);
}
.reply-composer {
  position: fixed;
  z-index: 70;
  inset: auto 20px 20px;
  display: grid;
  place-items: center;
  pointer-events: none;
}
.reply-composer form {
  width: min(620px, 100%);
  padding: 18px;
  border: 1px solid var(--color-line);
  border-radius: 18px;
  background: white;
  box-shadow: var(--shadow-lg);
  pointer-events: auto;
}
.reply-composer header {
  display: flex;
  justify-content: space-between;
  margin-bottom: 12px;
}
.reply-composer textarea {
  margin-bottom: 12px;
  padding: 12px;
  border: 1px solid var(--color-line);
  border-radius: 12px;
}
@media (max-width: 640px) {
  .comments__heading {
    align-items: start;
    flex-direction: column;
  }
  .comment-card {
    padding: 16px;
  }
  .reply-card {
    margin-left: 0;
  }
}
</style>
