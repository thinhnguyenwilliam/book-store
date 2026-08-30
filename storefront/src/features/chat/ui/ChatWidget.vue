<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useAuthStore } from '@/features/auth/model/auth.store'
import { useNotificationStore } from '@/features/notifications/model/notification.store'
import AppIcon from '@/shared/ui/AppIcon.vue'
import * as chatApi from '../api/chat.api'
import { ChatSocket } from '../lib/chat-socket'
import type { ChatEvent, ChatMessage, Conversation } from '../model/types'

const auth = useAuthStore()
const notifications = useNotificationStore()
const route = useRoute()
const router = useRouter()
const panelOpen = ref(false)
const loading = ref(false)
const connecting = ref(false)
const sending = ref(false)
const conversation = ref<Conversation>()
const messages = ref<ChatMessage[]>([])
const draft = ref('')
const unread = ref(0)
const connected = ref(false)
const typing = ref(false)
const listElement = ref<HTMLElement>()
const socket = new ChatSocket()
let reconnectTimer: number | undefined
let typingTimer: number | undefined
let reconnectAttempt = 0

watch(
  () => auth.isAuthenticated,
  async (authenticated) => {
    if (!authenticated) {
      socket.close()
      panelOpen.value = false
      conversation.value = undefined
      messages.value = []
      unread.value = 0
      return
    }
    unread.value = (await chatApi.unreadCount().catch(() => ({ count: 0 }))).count
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  window.clearTimeout(reconnectTimer)
  window.clearTimeout(typingTimer)
  socket.close()
})

async function togglePanel(): Promise<void> {
  if (!auth.isAuthenticated) {
    await router.push({ name: 'login', query: { redirect: route.fullPath } })
    return
  }
  panelOpen.value = !panelOpen.value
  if (panelOpen.value && !conversation.value) await initialize()
}

async function initialize(): Promise<void> {
  loading.value = true
  try {
    conversation.value = await chatApi.openSupportConversation()
    messages.value = (await chatApi.listMessages(conversation.value.id)).data
    await markLatestRead()
    await connect()
    await scrollToBottom()
  } catch {
    notifications.show('Chưa thể mở trò chuyện hỗ trợ.', 'error')
  } finally {
    loading.value = false
  }
}

async function connect(): Promise<void> {
  if (connecting.value || connected.value || !auth.isAuthenticated) return
  connecting.value = true
  try {
    await socket.connect(handleEvent, scheduleReconnect)
    connected.value = true
    reconnectAttempt = 0
  } catch {
    scheduleReconnect()
  } finally {
    connecting.value = false
  }
}

function scheduleReconnect(): void {
  connected.value = false
  if (!auth.isAuthenticated) return
  window.clearTimeout(reconnectTimer)
  const delay = Math.min(1000 * 2 ** reconnectAttempt, 15_000)
  reconnectAttempt += 1
  reconnectTimer = window.setTimeout(async () => {
    if (conversation.value) {
      messages.value = (
        await chatApi
          .listMessages(conversation.value.id)
          .catch(() => ({ data: messages.value, pagination: { has_more: false } }))
      ).data
    }
    await connect()
  }, delay)
}

function handleEvent(event: ChatEvent): void {
  if (
    event.type === 'message.created' ||
    event.type === 'message.updated' ||
    event.type === 'message.deleted'
  ) {
    const message = event.data as ChatMessage
    if (message.conversation_id !== conversation.value?.id) {
      if (event.type === 'message.created') unread.value += 1
      return
    }
    upsertMessage(message)
    if (panelOpen.value) void markLatestRead()
    else if (message.sender_id !== auth.profile?.id) unread.value += 1
    void scrollToBottom()
  }
  if (event.type === 'typing.changed') {
    const data = event.data as { conversation_id: string; user_id: string; active: boolean }
    if (data.conversation_id === conversation.value?.id && data.user_id !== auth.profile?.id) {
      typing.value = data.active
    }
  }
}

async function send(): Promise<void> {
  const content = draft.value.trim()
  if (!content || !conversation.value || sending.value) return
  const clientMessageId = crypto.randomUUID()
  sending.value = true
  draft.value = ''
  sendTyping(false)
  try {
    const sent = socket.send('message.send', {
      conversation_id: conversation.value.id,
      client_message_id: clientMessageId,
      content,
    })
    if (!sent)
      upsertMessage(await chatApi.sendMessage(conversation.value.id, content, clientMessageId))
  } catch {
    draft.value = content
    notifications.show('Không thể gửi tin nhắn.', 'error')
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
  if (!conversation.value) return
  socket.send('typing.changed', { conversation_id: conversation.value.id, active })
}

function upsertMessage(message: ChatMessage): void {
  const index = messages.value.findIndex(
    (item) => item.id === message.id || item.client_message_id === message.client_message_id,
  )
  if (index >= 0) messages.value[index] = message
  else messages.value.push(message)
  messages.value.sort((left, right) => left.sequence_number - right.sequence_number)
}

async function markLatestRead(): Promise<void> {
  const latest = messages.value.at(-1)
  if (!conversation.value || !latest) return
  unread.value = 0
  if (
    !socket.send('conversation.read', {
      conversation_id: conversation.value.id,
      sequence_number: latest.sequence_number,
    })
  ) {
    await chatApi.markRead(conversation.value.id, latest.sequence_number).catch(() => undefined)
  }
}

async function scrollToBottom(): Promise<void> {
  await nextTick()
  listElement.value?.scrollTo({ top: listElement.value.scrollHeight, behavior: 'smooth' })
}

function messageTime(value: string): string {
  return new Intl.DateTimeFormat('vi-VN', { hour: '2-digit', minute: '2-digit' }).format(
    new Date(value),
  )
}
</script>

<template>
  <div class="chat-widget">
    <Transition name="chat-panel">
      <section v-if="panelOpen" class="chat-panel" aria-label="Trò chuyện hỗ trợ">
        <header>
          <span
            ><strong>Hỗ trợ Mộc Thư</strong
            ><small
              ><i :class="{ online: connected }" />{{
                connected ? 'Đang trực tuyến' : 'Đang kết nối lại…'
              }}</small
            ></span
          >
          <button type="button" aria-label="Đóng trò chuyện" @click="panelOpen = false">
            <AppIcon name="close" />
          </button>
        </header>
        <div ref="listElement" class="chat-messages" aria-live="polite">
          <p v-if="loading" class="chat-state">Đang tải cuộc trò chuyện…</p>
          <p v-else-if="!messages.length" class="chat-welcome">
            Xin chào {{ auth.displayName }}! Bạn cần Mộc Thư hỗ trợ điều gì?
          </p>
          <article
            v-for="message in messages"
            :key="message.id"
            :class="{ mine: message.sender_id === auth.profile?.id }"
          >
            <span>{{ message.content }}</span>
            <time :datetime="message.created_at"
              >{{ messageTime(message.created_at)
              }}<em v-if="message.edited_at"> · đã sửa</em></time
            >
          </article>
          <p v-if="typing" class="chat-typing">Nhân viên đang nhập…</p>
        </div>
        <form @submit.prevent="send">
          <textarea
            v-model="draft"
            rows="1"
            maxlength="4000"
            placeholder="Nhập tin nhắn…"
            aria-label="Nội dung tin nhắn"
            @input="onDraftInput"
            @keydown.enter.exact.prevent="send"
          />
          <button type="submit" :disabled="!draft.trim() || sending" aria-label="Gửi tin nhắn">
            <AppIcon name="arrow-right" />
          </button>
        </form>
      </section>
    </Transition>
    <button
      class="chat-launcher"
      type="button"
      :aria-expanded="panelOpen"
      aria-label="Trò chuyện hỗ trợ"
      @click="togglePanel"
    >
      <AppIcon :name="panelOpen ? 'close' : 'chat'" :size="24" />
      <span v-if="unread">{{ unread > 99 ? '99+' : unread }}</span>
    </button>
  </div>
</template>

<style scoped>
.chat-widget {
  position: fixed;
  z-index: 70;
  right: 24px;
  bottom: 24px;
}
.chat-launcher {
  position: relative;
  display: grid;
  width: 58px;
  height: 58px;
  place-items: center;
  border: 0;
  border-radius: 50%;
  color: white;
  background: var(--color-brand);
  box-shadow: 0 14px 34px rgb(18 61 50 / 25%);
  cursor: pointer;
}
.chat-launcher span {
  position: absolute;
  top: -4px;
  right: -3px;
  min-width: 20px;
  padding: 3px 5px;
  border: 2px solid white;
  border-radius: 12px;
  background: #b94735;
  font-size: 0.62rem;
  font-weight: 800;
}
.chat-panel {
  position: absolute;
  right: 0;
  bottom: 72px;
  display: grid;
  width: min(380px, calc(100vw - 32px));
  height: min(570px, calc(100vh - 130px));
  grid-template-rows: auto 1fr auto;
  overflow: hidden;
  border: 1px solid var(--color-line);
  border-radius: 22px;
  background: var(--color-paper);
  box-shadow: 0 24px 70px rgb(16 48 40 / 24%);
}
.chat-panel header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 20px;
  color: white;
  background: var(--color-brand);
}
.chat-panel header span,
.chat-panel header strong,
.chat-panel header small {
  display: block;
}
.chat-panel header small {
  margin-top: 4px;
  opacity: 0.75;
  font-size: 0.68rem;
}
.chat-panel header i {
  display: inline-block;
  width: 7px;
  height: 7px;
  margin-right: 6px;
  border-radius: 50%;
  background: #c6a85c;
}
.chat-panel header i.online {
  background: #71d19c;
}
.chat-panel header button {
  border: 0;
  color: inherit;
  background: transparent;
  cursor: pointer;
}
.chat-messages {
  display: flex;
  flex-direction: column;
  gap: 9px;
  overflow-y: auto;
  padding: 18px;
}
.chat-messages article {
  align-self: flex-start;
  max-width: 82%;
}
.chat-messages article span {
  display: block;
  padding: 10px 13px;
  border-radius: 15px 15px 15px 4px;
  background: #ebe7dc;
  font-size: 0.82rem;
  line-height: 1.45;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.chat-messages article.mine {
  align-self: flex-end;
}
.chat-messages article.mine span {
  border-radius: 15px 15px 4px;
  color: white;
  background: var(--color-brand);
}
.chat-messages time {
  display: block;
  margin-top: 4px;
  color: var(--color-muted);
  font-size: 0.6rem;
}
.chat-messages article.mine time {
  text-align: right;
}
.chat-messages em {
  font-style: normal;
}
.chat-state,
.chat-welcome {
  margin: auto;
  color: var(--color-muted);
  font-size: 0.8rem;
  line-height: 1.55;
  text-align: center;
}
.chat-typing {
  margin: 2px 0;
  color: var(--color-muted);
  font-size: 0.68rem;
  font-style: italic;
}
.chat-panel form {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 8px;
  padding: 12px;
  border-top: 1px solid var(--color-line);
}
.chat-panel textarea {
  min-height: 42px;
  max-height: 100px;
  resize: none;
  border: 1px solid var(--color-line);
  border-radius: 13px;
  padding: 11px 13px;
  color: var(--color-ink);
  background: white;
  font: inherit;
  font-size: 0.8rem;
}
.chat-panel form button {
  width: 42px;
  height: 42px;
  border: 0;
  border-radius: 12px;
  color: white;
  background: var(--color-brand);
  cursor: pointer;
}
.chat-panel form button:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}
.chat-panel-enter-active,
.chat-panel-leave-active {
  transition:
    opacity 160ms ease,
    transform 160ms ease;
}
.chat-panel-enter-from,
.chat-panel-leave-to {
  opacity: 0;
  transform: translateY(12px) scale(0.98);
}
@media (max-width: 560px) {
  .chat-widget {
    right: 16px;
    bottom: 16px;
  }
  .chat-panel {
    position: fixed;
    inset: 12px 12px 84px;
    width: auto;
    height: auto;
  }
}
</style>
