<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { useAuthStore } from '@/features/auth/model/auth.store'
import { useCustomersStore } from '@/features/customers/model/customers.store'
import type { Customer, CustomerInput } from '@/features/customers/model/types'
import CustomerDrawer from '@/features/customers/ui/CustomerDrawer.vue'
import { useNotificationStore } from '@/features/notifications/model/notification.store'
import { formatDateTime, initials } from '@/shared/lib/format'
import AppIcon from '@/shared/ui/AppIcon.vue'
import ConfirmDialog from '@/shared/ui/ConfirmDialog.vue'

const store = useCustomersStore()
const auth = useAuthStore()
const notifications = useNotificationStore()
const query = ref('')
const selectedCustomer = ref<Customer>()
const deletingCustomer = ref<Customer>()

const filteredCustomers = computed(() => {
  const keyword = query.value.trim().toLocaleLowerCase('vi')
  if (!keyword) return store.customers
  return store.customers.filter((customer) =>
    [customer.display_name, customer.email, customer.id].some((value) =>
      value.toLocaleLowerCase('vi').includes(keyword),
    ),
  )
})

onMounted(() => store.fetchInitial())

async function save(payload: CustomerInput): Promise<void> {
  if (!selectedCustomer.value) return
  try {
    selectedCustomer.value = await store.update(selectedCustomer.value.id, payload)
    notifications.show('Đã cập nhật hồ sơ khách hàng.')
    selectedCustomer.value = undefined
  } catch {
    notifications.show(store.error || 'Không thể cập nhật khách hàng.', 'danger')
  }
}

async function confirmDelete(): Promise<void> {
  if (!deletingCustomer.value) return
  try {
    await store.remove(deletingCustomer.value.id)
    notifications.show(
      `Đã tiếp nhận yêu cầu xoá tài khoản “${deletingCustomer.value.display_name}”.`,
    )
    deletingCustomer.value = undefined
  } catch {
    notifications.show(store.error || 'Không thể xoá tài khoản khách hàng.', 'danger')
  }
}
</script>

<template>
  <div class="page-stack">
    <section class="page-heading">
      <div>
        <p class="eyebrow">Customer operations</p>
        <h2>Danh sách khách hàng</h2>
        <p>Xem và cập nhật hồ sơ khách hàng đã đăng ký trên storefront.</p>
      </div>
    </section>

    <section class="catalog-panel">
      <header class="catalog-toolbar">
        <label class="search-box">
          <AppIcon name="search" :size="18" />
          <span class="sr-only">Tìm kiếm khách hàng</span>
          <input v-model="query" type="search" placeholder="Tìm theo tên, email hoặc ID…" />
        </label>
        <div class="catalog-toolbar__meta">
          <span>{{ filteredCustomers.length }} hồ sơ đã tải</span>
          <button
            class="icon-button"
            type="button"
            title="Làm mới"
            :disabled="store.loading"
            @click="store.fetchInitial(true)"
          >
            <AppIcon name="refresh" :size="18" />
          </button>
        </div>
      </header>

      <p v-if="store.error" class="inline-error catalog-error">{{ store.error }}</p>

      <div class="table-wrap">
        <table class="books-table customers-table">
          <thead>
            <tr>
              <th>Khách hàng</th>
              <th>Customer ID</th>
              <th>Ngày tham gia</th>
              <th>Cập nhật</th>
              <th><span class="sr-only">Thao tác</span></th>
            </tr>
          </thead>
          <tbody v-if="store.loading">
            <tr v-for="item in 6" :key="item" class="table-skeleton">
              <td colspan="5"><i /></td>
            </tr>
          </tbody>
          <tbody v-else-if="filteredCustomers.length">
            <tr v-for="customer in filteredCustomers" :key="customer.id">
              <td>
                <div class="customer-cell">
                  <span class="customer-avatar">{{
                    initials(customer.display_name || customer.email)
                  }}</span>
                  <p>
                    <strong>{{ customer.display_name }}</strong
                    ><small>{{ customer.email }}</small>
                  </p>
                </div>
              </td>
              <td>
                <code>{{ customer.id }}</code>
              </td>
              <td>
                <span class="date-cell">{{ formatDateTime(customer.created_at) }}</span>
              </td>
              <td>
                <span class="date-cell">{{ formatDateTime(customer.updated_at) }}</span>
              </td>
              <td>
                <div class="row-actions">
                  <button
                    type="button"
                    title="Xem và sửa hồ sơ"
                    @click="selectedCustomer = customer"
                  >
                    <AppIcon name="edit" :size="17" />
                  </button>
                  <button
                    class="row-actions__delete"
                    type="button"
                    :disabled="customer.id === auth.profile?.id"
                    :title="
                      customer.id === auth.profile?.id
                        ? 'Không thể xoá tài khoản đang đăng nhập'
                        : 'Xoá tài khoản'
                    "
                    @click="deletingCustomer = customer"
                  >
                    <AppIcon name="trash" :size="17" />
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>

        <div v-if="!store.loading && !filteredCustomers.length" class="empty-state">
          <span><AppIcon name="user" :size="26" /></span>
          <h3>{{ query ? 'Không tìm thấy khách hàng' : 'Chưa có khách hàng' }}</h3>
          <p>{{ query ? 'Thử một từ khóa khác.' : 'Hồ sơ sẽ xuất hiện sau khi khách đăng ký.' }}</p>
        </div>
      </div>

      <footer v-if="store.hasMore && !query" class="catalog-footer">
        <button
          class="button button--secondary"
          type="button"
          :disabled="store.loadingMore"
          @click="store.fetchNext"
        >
          <span v-if="store.loadingMore" class="spinner" />
          {{ store.loadingMore ? 'Đang tải…' : 'Tải thêm khách hàng' }}
        </button>
      </footer>
    </section>

    <CustomerDrawer
      :open="Boolean(selectedCustomer)"
      :customer="selectedCustomer"
      :saving="store.saving"
      @close="selectedCustomer = undefined"
      @save="save"
    />
    <ConfirmDialog
      :open="Boolean(deletingCustomer)"
      title="Xoá tài khoản khách hàng?"
      :message="`Tài khoản “${deletingCustomer?.display_name || ''}” sẽ không thể đăng nhập hoặc refresh token. Hồ sơ sẽ được xoá bất đồng bộ qua RabbitMQ.`"
      :busy="Boolean(store.deletingId)"
      @cancel="deletingCustomer = undefined"
      @confirm="confirmDelete"
    />
  </div>
</template>

<style scoped>
.customers-table {
  min-width: 920px;
}
.customer-cell {
  display: flex;
  min-width: 220px;
  gap: 12px;
  align-items: center;
}
.customer-avatar {
  display: grid;
  width: 40px;
  height: 40px;
  flex: 0 0 auto;
  place-items: center;
  border-radius: 50%;
  color: #205648;
  background: #e3f0eb;
  font-size: 0.7rem;
  font-weight: 850;
}
.customer-cell p {
  min-width: 0;
  margin: 0;
}
.customer-cell strong,
.customer-cell small {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.customer-cell strong {
  font-size: 0.8rem;
}
.customer-cell small {
  margin-top: 4px;
  color: var(--color-muted);
  font-size: 0.67rem;
}
.customers-table code {
  display: block;
  max-width: 190px;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
