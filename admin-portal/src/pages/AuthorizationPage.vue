<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useAuthStore } from '@/features/auth/model/auth.store'
import * as api from '@/features/authorization/api'
import type { Access, Audit, Permission, Role } from '@/features/authorization/api'
import { canGrantRole } from '@/features/authorization/landing'

const auth = useAuthStore()
const roles = ref<Role[]>([])
const permissions = ref<Permission[]>([])
const entries = ref<Audit[]>([])
const error = ref('')
const message = ref('')
const busy = ref(false)
const editing = ref<Role>()
const creating = ref(false)
const accountId = ref('')
const account = ref<Access>()
const selectedRoles = ref<string[]>([])
const groups = computed(() => [...new Set(permissions.value.map((p) => p.group))])
const writable = computed(
  () =>
    auth.can('roles.manage') &&
    !editing.value?.system &&
    !auth.roles.includes(editing.value?.code || ''),
)

async function run(action: () => Promise<void>): Promise<void> {
  if (busy.value) return
  busy.value = true
  error.value = ''
  message.value = ''
  try {
    await action()
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : 'Không thể thực hiện yêu cầu.'
  } finally {
    busy.value = false
  }
}
async function load(): Promise<void> {
  const result = await api.catalog()
  roles.value = result.roles
  permissions.value = result.permissions
  entries.value = await api.audit()
}
onMounted(() => run(load))
function edit(role?: Role): void {
  creating.value = !role
  editing.value = role
    ? { ...role, permissions: [...role.permissions] }
    : { code: '', name: '', description: '', system: false, permissions: [] }
}
async function save(): Promise<void> {
  if (!editing.value || !writable.value) return
  await run(async () => {
    await api.saveRole(editing.value!, creating.value)
    editing.value = undefined
    await load()
    await auth.refreshPermissions()
    message.value = 'Đã lưu vai trò và ghi lịch sử thay đổi.'
  })
}
async function lookup(): Promise<void> {
  account.value = undefined
  selectedRoles.value = []
  await run(async () => {
    account.value = await api.accountAccess(accountId.value.trim())
    selectedRoles.value = [...account.value.roles]
  })
}
async function assign(): Promise<void> {
  if (!account.value || !window.confirm('Thay thế toàn bộ vai trò của tài khoản này?')) return
  const id = account.value.account_id
  await run(async () => {
    await api.assignRoles(id, selectedRoles.value)
    account.value = await api.accountAccess(id)
    await load()
    message.value = 'Đã cập nhật quyền của tài khoản.'
  })
}
async function removeRole(): Promise<void> {
  if (!editing.value || !writable.value) return
  if (
    !window.confirm(
      `Xóa vai trò “${editing.value.name}”? Chỉ xóa được khi không còn tài khoản nào đang mang vai trò này.`,
    )
  )
    return
  const code = editing.value.code
  await run(async () => {
    await api.deleteRole(code)
    editing.value = undefined
    await load()
    message.value = 'Đã xóa vai trò tùy chỉnh.'
  })
}
async function moreAudit(): Promise<void> {
  const last = entries.value.at(-1)
  if (last)
    await run(async () => {
      entries.value.push(...(await api.audit(last.id)))
    })
}
</script>

<template>
  <div class="page-stack authorization-page">
    <header class="page-heading">
      <div>
        <p class="eyebrow">Access control</p>
        <h2>Vai trò & phân quyền</h2>
        <p>Quyền hiệu lực là tổng quyền của các vai trò. Backend kiểm tra mọi thao tác quản trị.</p>
      </div>
    </header>
    <p v-if="error" role="alert" class="notice error">{{ error }}</p>
    <p v-if="message" role="status" class="notice">{{ message }}</p>
    <section class="catalog-panel panel">
      <header class="section-heading">
        <h3>Danh sách vai trò</h3>
        <button
          v-if="auth.can('roles.manage')"
          class="button button--primary"
          :disabled="busy"
          @click="edit()"
        >
          Tạo vai trò
        </button>
      </header>
      <div class="role-grid">
        <button v-for="role in roles" :key="role.code" class="role-card" @click="edit(role)">
          <strong>{{ role.name }}</strong
          ><code>{{ role.code }}</code
          ><span
            >{{ role.permissions.length }} quyền ·
            {{ role.system ? 'Hệ thống · chỉ xem' : 'Tùy chỉnh' }}</span
          >
        </button>
      </div>
      <form v-if="editing" class="editor" @submit.prevent="save">
        <h3>{{ creating ? 'Tạo vai trò mới' : editing.name }}</h3>
        <label
          >Mã vai trò<input
            v-model="editing.code"
            required
            pattern="[a-z][a-z0-9_]{2,63}"
            maxlength="64"
            :disabled="!creating"
            placeholder="warehouse_staff"
        /></label>
        <label
          >Tên<input v-model="editing.name" required maxlength="120" :disabled="!writable"
        /></label>
        <label
          >Mô tả<input v-model="editing.description" maxlength="1000" :disabled="!writable"
        /></label>
        <fieldset v-for="group in groups" :key="group" :disabled="!writable || busy">
          <legend>{{ group }}</legend>
          <label
            v-for="permission in permissions.filter((p) => p.group === group)"
            :key="permission.code"
            class="check"
            ><input
              v-model="editing.permissions"
              type="checkbox"
              :value="permission.code"
              :disabled="!auth.can(permission.code)"
            /><span
              >{{ permission.name }} <code>{{ permission.code }}</code></span
            ></label
          >
        </fieldset>
        <p v-if="!writable">Vai trò hệ thống hoặc vai trò của chính bạn chỉ được xem.</p>
        <div v-if="writable" class="editor-actions">
          <button class="button button--primary" :disabled="busy">Lưu vai trò</button>
          <button
            v-if="!creating"
            type="button"
            class="button button--secondary"
            :disabled="busy"
            @click="removeRole"
          >
            Xóa vai trò
          </button>
        </div>
        <button type="button" class="button button--secondary" @click="editing = undefined">
          Đóng
        </button>
      </form>
    </section>
    <section class="catalog-panel panel">
      <h3>Quyền của tài khoản</h3>
      <p>
        Nhập ID từ danh sách khách hàng để xem quyền hoặc gán vai trò. Không được thay đổi quyền của
        chính mình.
      </p>
      <form class="lookup" @submit.prevent="lookup">
        <input
          v-model="accountId"
          required
          placeholder="UUID tài khoản"
          aria-label="ID tài khoản"
        /><button class="button button--secondary" :disabled="busy">Tra cứu</button>
      </form>
      <div v-if="account">
        <p>
          <code>{{ account.account_id }}</code>
        </p>
        <div class="role-grid">
          <label v-for="role in roles" :key="role.code" class="check"
            ><input
              v-model="selectedRoles"
              type="checkbox"
              :value="role.code"
              :disabled="
                busy ||
                !auth.can('users.assign_roles') ||
                account.account_id === auth.profile?.id ||
                !canGrantRole(role, auth.roles, auth.permissions)
              "
            />{{ role.name }}</label
          >
        </div>
        <p>Quyền hiện hành: {{ account.permissions.join(', ') || 'Không có quyền quản trị' }}</p>
        <button
          v-if="auth.can('users.assign_roles')"
          class="button button--primary"
          :disabled="busy || !selectedRoles.length || account.account_id === auth.profile?.id"
          @click="assign"
        >
          Lưu vai trò tài khoản
        </button>
      </div>
    </section>
    <section class="catalog-panel panel">
      <h3>Lịch sử phân quyền</h3>
      <p v-if="!entries.length">Chưa có thay đổi.</p>
      <details v-for="entry in entries" :key="entry.id">
        <summary>
          {{ new Date(entry.created_at).toLocaleString('vi-VN') }} · {{ entry.action }} ·
          {{ entry.target }}
        </summary>
        <p>Người thực hiện: {{ entry.actor_id }} · Trace: {{ entry.trace_id || '—' }}</p>
        <pre>
Trước: {{ entry.before }}
Sau: {{ entry.after }}</pre>
      </details>
      <button
        v-if="entries.length >= 50"
        class="button button--secondary"
        :disabled="busy"
        @click="moreAudit"
      >
        Xem lịch sử cũ hơn
      </button>
    </section>
  </div>
</template>

<style scoped>
.panel {
  padding: 1.5rem;
}
.section-heading,
.lookup {
  display: flex;
  gap: 1rem;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
}
.role-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 0.8rem;
  margin: 1rem 0;
}
.role-card {
  text-align: left;
  padding: 1rem;
  display: grid;
  gap: 0.5rem;
  border: 1px solid #d6dfd9;
  border-radius: 10px;
  background: #fafbf8;
  cursor: pointer;
}
.editor {
  display: grid;
  gap: 1rem;
  margin-top: 1.5rem;
}
.editor > label {
  display: grid;
  gap: 0.4rem;
}
.editor-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}
input:not([type='checkbox']) {
  padding: 0.7rem;
  border: 1px solid #ccd6cf;
  border-radius: 6px;
}
fieldset {
  border: 1px solid #d6dfd9;
  border-radius: 8px;
}
.check {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.5rem;
}
code {
  font-size: 0.8rem;
  overflow-wrap: anywhere;
}
.check code {
  display: block;
}
.notice {
  padding: 1rem;
  background: #e9f4ed;
  border-radius: 8px;
}
.error {
  background: #fbe9e6;
}
details {
  padding: 1rem 0;
  border-bottom: 1px solid #ddd;
}
pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
</style>
