<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'

import * as chatApi from '@/features/chat/api/chat.api'
import { ChatSocket } from '@/features/chat/lib/chat-socket'
import type { ChatEvent, ChatMessage, Conversation } from '@/features/chat/model/types'
import { useAuthStore } from '@/features/auth/model/auth.store'
import { useNotificationStore } from '@/features/notifications/model/notification.store'

const auth = useAuthStore()
const notifications = useNotificationStore()
const conversations = ref<Conversation[]>([])
const selected = ref<Conversation>()
const messages = ref<ChatMessage[]>([])
const loading = ref(true)
const loadingMessages = ref(false)
const sending = ref(false)
const connected = ref(false)
const customerTyping = ref(false)
const draft = ref('')
const messageList = ref<HTMLElement>()
const socket = new ChatSocket()
let reconnectTimer: number | undefined
let reconnectAttempt = 0
let typingTimer: number | undefined

onMounted(async () => {
  await refreshConversations()
  await connect()
})

onBeforeUnmount(() => {
  window.clearTimeout(reconnectTimer)
  window.clearTimeout(typingTimer)
  socket.close()
})

async function refreshConversations(): Promise<void> {
  try {
    conversations.value = (await chatApi.listConversations()).data
    if (selected.value) {
      selected.value = conversations.value.find((item) => item.id === selected.value?.id)
    }
  } catch {
    notifications.show('Không thể tải danh sách trò chuyện.', 'danger')
  } finally {
    loading.value = false
  }
}

async function selectConversation(item: Conversation): Promise<void> {
  selected.value = item
  loadingMessages.value = true
  customerTyping.value = false
  try {
    messages.value = (await chatApi.listMessages(item.id)).data
    await markLatestRead()
    await scrollToBottom()
  } catch {
    notifications.show('Không thể tải nội dung trò chuyện.', 'danger')
  } finally {
    loadingMessages.value = false
  }
}

async function connect(): Promise<void> {
  try {
    await socket.connect(handleEvent, scheduleReconnect)
    connected.value = true
    reconnectAttempt = 0
  } catch {
    scheduleReconnect()
  }
}

function scheduleReconnect(): void {
  connected.value = false
  window.clearTimeout(reconnectTimer)
  const delay = Math.min(1000 * 2 ** reconnectAttempt, 15_000)
  reconnectAttempt += 1
  reconnectTimer = window.setTimeout(async () => {
    await refreshConversations()
    if (selected.value) await selectConversation(selected.value)
    await connect()
  }, delay)
}

function handleEvent(event: ChatEvent): void {
  if (event.type.startsWith('message.')) {
    const message = event.data as ChatMessage
    if (message.conversation_id === selected.value?.id) {
      upsertMessage(message)
      void markLatestRead()
      void scrollToBottom()
    }
    void refreshConversations()
  }
  if (event.type === 'conversation.updated') void refreshConversations()
  if (event.type === 'typing.changed') {
    const data = event.data as { conversation_id: string; user_id: string; active: boolean }
    if (data.conversation_id === selected.value?.id && data.user_id !== auth.profile?.id) {
      customerTyping.value = data.active
    }
  }
}

async function send(): Promise<void> {
  const content = draft.value.trim()
  if (!selected.value || !content || sending.value) return
  const clientMessageId = crypto.randomUUID()
  sending.value = true
  draft.value = ''
  sendTyping(false)
  try {
    const sent = socket.send('message.send', {
      conversation_id: selected.value.id,
      client_message_id: clientMessageId,
      content,
    })
    if (!sent) upsertMessage(await chatApi.sendMessage(selected.value.id, content, clientMessageId))
  } catch {
    draft.value = content
    notifications.show('Không thể gửi tin nhắn.', 'danger')
  } finally {
    sending.value = false
  }
}

function onDraftInput(): void {
  sendTyping(true)
  window.clearTimeout(typingTimer)
  typingTimer = window.setTimeout(() => sendTyping(false), 1500)
}

function sendTyping(active: boolean): void {
  if (!selected.value) return
  socket.send('typing.changed', { conversation_id: selected.value.id, active })
}

async function markLatestRead(): Promise<void> {
  const latest = messages.value.at(-1)
  if (!selected.value || !latest) return
  selected.value.unread_count = 0
  if (
    !socket.send('conversation.read', {
      conversation_id: selected.value.id,
      sequence_number: latest.sequence_number,
    })
  ) {
    await chatApi.markRead(selected.value.id, latest.sequence_number).catch(() => undefined)
  }
}

function upsertMessage(message: ChatMessage): void {
  const index = messages.value.findIndex(
    (item) => item.id === message.id || item.client_message_id === message.client_message_id,
  )
  if (index >= 0) messages.value[index] = message
  else messages.value.push(message)
  messages.value.sort((left, right) => left.sequence_number - right.sequence_number)
}

async function scrollToBottom(): Promise<void> {
  await nextTick()
  messageList.value?.scrollTo({ top: messageList.value.scrollHeight, behavior: 'smooth' })
}

function shortID(value: string): string {
  return value.slice(0, 8)
}
function formatTime(value?: string): string {
  if (!value) return 'Chưa có tin nhắn'
  return new Intl.DateTimeFormat('vi-VN', { dateStyle: 'short', timeStyle: 'short' }).format(
    new Date(value),
  )
}
</script>

<template>
  <div class="page-stack">
    <section class="page-heading chat-heading">
      <div>
        <p class="eyebrow">Customer care</p>
        <h2>Trò chuyện hỗ trợ</h2>
        <p>Phản hồi khách hàng theo thời gian thực.</p>
      </div>
      <span class="connection-state" :class="{ online: connected }"
        ><i />{{ connected ? 'Realtime đang hoạt động' : 'Đang kết nối lại' }}</span
      >
    </section>
    <section class="chat-workspace">
      <aside class="conversation-list">
        <header>
          <strong>Hộp thư</strong><span>{{ conversations.length }} cuộc trò chuyện</span>
        </header>
        <p v-if="loading" class="chat-empty">Đang tải…</p>
        <p v-else-if="!conversations.length" class="chat-empty">Chưa có yêu cầu hỗ trợ.</p>
        <button
          v-for="item in conversations"
          :key="item.id"
          type="button"
          :class="{ active: selected?.id === item.id }"
          @click="selectConversation(item)"
        >
          <span class="conversation-avatar">{{
            shortID(item.customer_id).slice(0, 2).toUpperCase()
          }}</span>
          <span
            ><strong>Khách #{{ shortID(item.customer_id) }}</strong
            ><small>{{ item.last_message_preview || 'Cuộc trò chuyện mới' }}</small
            ><time>{{ formatTime(item.last_message_at) }}</time></span
          >
          <em v-if="item.unread_count">{{ item.unread_count > 99 ? '99+' : item.unread_count }}</em>
        </button>
      </aside>
      <div class="conversation-panel">
        <div v-if="!selected" class="chat-placeholder">
          <span>💬</span>
          <h3>Chọn một cuộc trò chuyện</h3>
          <p>Tin nhắn mới sẽ xuất hiện tại đây theo thời gian thực.</p>
        </div>
        <template v-else>
          <header>
            <div>
              <strong>Khách hàng #{{ shortID(selected.customer_id) }}</strong
              ><small>Customer ID: {{ selected.customer_id }}</small>
            </div>
            <span>{{ selected.status === 'open' ? 'Đang mở' : 'Đã đóng' }}</span>
          </header>
          <div ref="messageList" class="message-list">
            <p v-if="loadingMessages" class="chat-empty">Đang tải tin nhắn…</p>
            <article
              v-for="message in messages"
              v-else
              :key="message.id"
              :class="{ mine: message.sender_id === auth.profile?.id }"
            >
              <small>{{ message.sender_name }}</small>
              <p>{{ message.content }}</p>
              <time
                >{{ formatTime(message.created_at)
                }}<em v-if="message.edited_at"> · đã sửa</em></time
              >
            </article>
            <p v-if="customerTyping" class="typing-state">Khách hàng đang nhập…</p>
          </div>
          <form @submit.prevent="send">
            <textarea
              v-model="draft"
              rows="2"
              maxlength="4000"
              placeholder="Nhập câu trả lời…"
              @input="onDraftInput"
              @keydown.enter.exact.prevent="send"
            /><button
              class="button button--primary"
              type="submit"
              :disabled="!draft.trim() || sending"
            >
              {{ sending ? 'Đang gửi…' : 'Gửi tin nhắn' }}
            </button>
          </form>
        </template>
      </div>
    </section>
  </div>
</template>

<style scoped>
.chat-heading {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
}
.connection-state {
  display: inline-flex;
  gap: 7px;
  align-items: center;
  color: var(--color-muted);
  font-size: 0.72rem;
  font-weight: 700;
}
.connection-state i {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #c39b4d;
}
.connection-state.online i {
  background: #2c9a68;
}
.chat-workspace {
  display: grid;
  min-height: 650px;
  grid-template-columns: 330px 1fr;
  overflow: hidden;
  border: 1px solid var(--color-line);
  border-radius: 18px;
  background: white;
  box-shadow: var(--shadow-sm);
}
.conversation-list {
  overflow-y: auto;
  border-right: 1px solid var(--color-line);
}
.conversation-list > header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px;
  border-bottom: 1px solid var(--color-line);
}
.conversation-list > header span {
  color: var(--color-muted);
  font-size: 0.65rem;
}
.conversation-list > button {
  position: relative;
  display: grid;
  width: 100%;
  grid-template-columns: auto 1fr auto;
  gap: 11px;
  padding: 15px 17px;
  border: 0;
  border-bottom: 1px solid var(--color-line);
  color: inherit;
  background: white;
  text-align: left;
  cursor: pointer;
}
.conversation-list > button:hover,
.conversation-list > button.active {
  background: #f0f6f3;
}
.conversation-avatar {
  display: grid;
  width: 38px;
  height: 38px;
  place-items: center;
  border-radius: 50%;
  color: #205648;
  background: #dcebe5;
  font-size: 0.7rem;
  font-weight: 850;
}
.conversation-list button strong,
.conversation-list button small,
.conversation-list button time {
  display: block;
}
.conversation-list button strong {
  font-size: 0.76rem;
}
.conversation-list button small {
  overflow: hidden;
  margin-top: 4px;
  color: var(--color-muted);
  font-size: 0.66rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.conversation-list button time {
  margin-top: 5px;
  color: var(--color-muted);
  font-size: 0.58rem;
}
.conversation-list button em {
  min-width: 20px;
  padding: 3px 5px;
  border-radius: 10px;
  color: white;
  background: #b94735;
  font-size: 0.58rem;
  font-style: normal;
  text-align: center;
}
.conversation-panel {
  display: grid;
  min-width: 0;
  grid-template-rows: auto 1fr auto;
}
.conversation-panel > header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 17px 22px;
  border-bottom: 1px solid var(--color-line);
}
.conversation-panel > header strong,
.conversation-panel > header small {
  display: block;
}
.conversation-panel > header small {
  margin-top: 4px;
  color: var(--color-muted);
  font-size: 0.62rem;
}
.conversation-panel > header > span {
  padding: 5px 9px;
  border-radius: 12px;
  color: #247050;
  background: #e1f2e9;
  font-size: 0.62rem;
  font-weight: 750;
}
.message-list {
  display: flex;
  flex-direction: column;
  gap: 13px;
  overflow-y: auto;
  padding: 24px;
  background: #fafbfa;
}
.message-list article {
  align-self: flex-start;
  max-width: 72%;
}
.message-list article > small {
  display: block;
  margin: 0 0 4px 3px;
  color: var(--color-muted);
  font-size: 0.62rem;
}
.message-list article p {
  margin: 0;
  padding: 11px 14px;
  border: 1px solid var(--color-line);
  border-radius: 14px 14px 14px 4px;
  background: white;
  font-size: 0.78rem;
  line-height: 1.5;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.message-list article > time {
  display: block;
  margin-top: 4px;
  color: var(--color-muted);
  font-size: 0.57rem;
}
.message-list article.mine {
  align-self: flex-end;
}
.message-list article.mine > small,
.message-list article.mine > time {
  text-align: right;
}
.message-list article.mine p {
  border-color: #205648;
  border-radius: 14px 14px 4px;
  color: white;
  background: #205648;
}
.message-list time em {
  font-style: normal;
}
.typing-state {
  color: var(--color-muted);
  font-size: 0.67rem;
  font-style: italic;
}
.conversation-panel form {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 12px;
  padding: 16px;
  border-top: 1px solid var(--color-line);
}
.conversation-panel textarea {
  resize: none;
  border: 1px solid var(--color-line);
  border-radius: 12px;
  padding: 11px 13px;
  font: inherit;
}
.conversation-panel form button {
  align-self: stretch;
}
.chat-placeholder {
  display: grid;
  align-content: center;
  justify-items: center;
  padding: 40px;
  text-align: center;
}
.chat-placeholder span {
  font-size: 2rem;
}
.chat-placeholder h3 {
  margin: 12px 0 6px;
}
.chat-placeholder p,
.chat-empty {
  color: var(--color-muted);
  font-size: 0.75rem;
}
.chat-empty {
  padding: 22px;
  text-align: center;
}
@media (max-width: 900px) {
  .chat-workspace {
    grid-template-columns: 280px 1fr;
  }
}
@media (max-width: 720px) {
  .chat-heading {
    align-items: flex-start;
    flex-direction: column;
    gap: 12px;
  }
  .chat-workspace {
    display: block;
  }
  .conversation-list {
    max-height: 280px;
    border-right: 0;
    border-bottom: 1px solid var(--color-line);
  }
  .conversation-panel {
    min-height: 560px;
  }
}
</style>
