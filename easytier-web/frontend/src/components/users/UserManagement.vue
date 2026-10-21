<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import Button from 'primevue/button';
import Dialog from 'primevue/dialog';
import InputText from 'primevue/inputtext';
import type { CredentialDelivery, CurrentUser, Page, UserEvent, UserListFilter, UserSummary } from './types';

const props = defineProps<{
    currentUser: CurrentUser;
    users: Page<UserSummary>;
    events: Page<UserEvent>;
    selectedUser: UserSummary | null;
    loading: boolean;
    busy: boolean;
    stale: boolean;
    error: { code: string; message: string } | null;
    credential: CredentialDelivery | null;
}>();
const emit = defineEmits<{
    query: [filter: UserListFilter]; select: [id: number]; create: [input: { username: string }];
    'set-access': [input: { id: number; web_login_enabled: boolean }];
    'set-role': [input: { id: number; role: 'admin' | 'user' }];
    'reset-password': [id: number]; 'revoke-sessions': [id: number];
    'query-events': [input: { target_id?: number; page: number; page_size: number }];
    'dismiss-credential': []; retry: [];
}>();
const { t } = useI18n();
const search = ref('');
const role = ref<'' | 'admin' | 'user'>('');
const access = ref('');
const username = ref('');
const createVisible = ref(false);
const submitted = ref(false);
const revealed = ref(false);
const canManage = computed(() => props.currentUser.capabilities.includes('users.manage') && !props.currentUser.must_change_password);
const disabled = computed(() => props.busy || submitted.value || !canManage.value);
const usernameValid = computed(() => /^[A-Za-z0-9_.@-]{3,64}$/.test(username.value));
type Action = 'access' | 'role' | 'password_reset' | 'revoke_sessions';
const pending = ref<{ action: Action; user: UserSummary } | null>(null);
const confirmVisible = computed({ get: () => pending.value !== null, set: value => { if (!value) pending.value = null; } });
const credentialVisible = computed({ get: () => props.credential !== null, set: value => { if (!value) emit('dismiss-credential'); } });
watch(() => props.busy, (busy, previous) => {
    if (!busy && previous) { submitted.value = false; if (!props.error) { createVisible.value = false; pending.value = null; } }
});
watch(() => props.error, error => { if (error) submitted.value = false; });
watch(() => props.credential, () => { revealed.value = false; submitted.value = false; });
watch(canManage, allowed => { if (!allowed) { pending.value = null; createVisible.value = false; emit('dismiss-credential'); } });
function query(page = 1) {
    if (props.loading) return;
    emit('query', { search: search.value || undefined, role: role.value || undefined,
        web_login_enabled: access.value === '' ? undefined : access.value === 'enabled', page, page_size: props.users.page_size });
}
function create() {
    if (disabled.value || !usernameValid.value) return;
    submitted.value = true;
    emit('create', { username: username.value });
}
function confirm() {
    if (!pending.value || disabled.value) return;
    const { action, user } = pending.value;
    submitted.value = true;
    if (action === 'access') emit('set-access', { id: user.id, web_login_enabled: !user.web_login_enabled });
    if (action === 'role') emit('set-role', { id: user.id, role: user.role === 'admin' ? 'user' : 'admin' });
    if (action === 'password_reset') emit('reset-password', user.id);
    if (action === 'revoke_sessions') emit('revoke-sessions', user.id);
}
function choose(action: Action, user: UserSummary) { if (!disabled.value) pending.value = { action, user: { ...user } }; }
function select(user: UserSummary) {
    emit('select', user.id);
    emit('query-events', { target_id: user.id, page: 1, page_size: props.events.page_size });
}
function eventPage(page: number) { emit('query-events', { target_id: props.selectedUser?.id, page, page_size: props.events.page_size }); }
function date(value: string | null) { return value ?? t('user_management.unknown'); }
const knownErrors = ['forbidden', 'password_change_required', 'self_action_forbidden', 'local_password_unavailable', 'csrf_rejected', 'user_not_found', 'username_conflict', 'last_admin', 'identity_conflict', 'unauthenticated', 'network_error', 'invalid_request', 'internal_error'];
const errorText = computed(() => props.error ? t(`user_management.errors.${knownErrors.includes(props.error.code) ? props.error.code : 'internal_error'}`) : '');
</script>

<template>
    <section class="user-management" :aria-busy="loading || busy">
        <header><h1>{{ t('user_management.title') }}</h1><Button :label="t('user_management.create')" :disabled="disabled" @click="createVisible = true" /></header>
        <p>{{ t('user_management.scope') }}</p>
        <p v-if="!canManage" role="alert">{{ t('user_management.errors.forbidden') }}</p>
        <div v-if="error" role="alert"><p>{{ errorText }}</p><Button :label="t('user_management.retry')" :disabled="loading || busy" @click="emit('retry')" /></div>
        <p v-if="stale" role="status">{{ t('user_management.stale') }}</p>
        <form class="filters" @submit.prevent="query()">
            <label>{{ t('user_management.search') }}<InputText v-model="search" :disabled="loading" /></label>
            <label>{{ t('user_management.role') }}<select v-model="role" :aria-label="t('user_management.role')" :disabled="loading"><option value="">{{ t('user_management.all') }}</option><option value="user">{{ t('user_management.roles.user') }}</option><option value="admin">{{ t('user_management.roles.admin') }}</option></select></label>
            <label>{{ t('user_management.access') }}<select v-model="access" :aria-label="t('user_management.access')" :disabled="loading"><option value="">{{ t('user_management.all') }}</option><option value="enabled">{{ t('user_management.enabled') }}</option><option value="disabled">{{ t('user_management.disabled') }}</option></select></label>
            <Button type="submit" :label="t('user_management.search')" :disabled="loading || !canManage" />
        </form>
        <p v-if="loading" role="status">{{ t('user_management.loading') }}</p>
        <p v-else-if="!error && !users.items.length" role="status">{{ t(search || role || access ? 'user_management.no_matches' : 'user_management.empty') }}</p>
        <div v-if="users.items.length" class="table-scroll" role="region" :aria-label="t('user_management.title')" tabindex="0"><table><caption class="sr-only">{{ t('user_management.title') }}</caption><thead><tr><th>{{ t('user_management.username') }}</th><th>{{ t('user_management.role') }}</th><th>{{ t('user_management.access') }}</th><th>{{ t('user_management.source') }}</th><th>{{ t('user_management.created') }}</th><th>{{ t('user_management.last_login') }}</th><th>{{ t('user_management.details') }}</th></tr></thead>
            <tbody><tr v-for="user in users.items" :key="user.id"><td class="username">{{ user.username }}</td><td>{{ t(`user_management.roles.${user.role}`) }}</td><td>{{ t(user.web_login_enabled ? 'user_management.enabled' : 'user_management.disabled') }}</td><td>{{ t(`user_management.sources.${user.source}`) }}</td><td>{{ date(user.created_at) }}</td><td>{{ date(user.last_login_at) }}</td><td><Button :label="t('user_management.details')" :aria-label="`${t('user_management.details')}: ${user.username}`" :disabled="busy || !canManage" @click="select(user)" /></td></tr></tbody></table></div>
        <nav class="pagination" :aria-label="t('user_management.pages')"><Button :label="t('user_management.previous')" :disabled="loading || users.page <= 1 || !canManage" @click="query(users.page - 1)" /><span>{{ t('user_management.page', { page: users.page, total: users.total }) }}</span><Button :label="t('user_management.next')" :disabled="loading || users.page * users.page_size >= users.total || !canManage" @click="query(users.page + 1)" /></nav>
        <section v-if="selectedUser" class="details" :aria-label="t('user_management.details')"><h2 class="username">{{ selectedUser.username }}</h2><p>{{ t('user_management.scope') }}</p>
            <div class="actions"><Button :label="t(selectedUser.web_login_enabled ? 'user_management.disable' : 'user_management.enable')" :disabled="disabled || selectedUser.id === currentUser.id" @click="choose('access', selectedUser)" />
                <Button :label="t(selectedUser.role === 'admin' ? 'user_management.make_user' : 'user_management.make_admin')" :disabled="disabled || selectedUser.id === currentUser.id" @click="choose('role', selectedUser)" />
                <Button :label="t('user_management.reset')" :disabled="disabled || selectedUser.local_password_enabled !== true || selectedUser.id === currentUser.id" @click="choose('password_reset', selectedUser)" />
                <Button :label="t('user_management.revoke')" :disabled="disabled" @click="choose('revoke_sessions', selectedUser)" /></div>
            <p v-if="selectedUser.local_password_enabled !== true">{{ t('user_management.errors.local_password_unavailable') }}</p><p v-if="selectedUser.id === currentUser.id">{{ t('user_management.self_protection') }}</p>
            <h3>{{ t('user_management.audit') }}</h3><ul class="audit"><li v-for="event in events.items" :key="event.id">{{ date(event.occurred_at) }} · {{ t(`user_management.audit_actions.${event.action}`) }} · {{ t(`user_management.results.${event.result}`) }} · {{ t('user_management.actor') }} {{ event.actor_id ?? t('user_management.offline_operator') }}</li></ul><p v-if="!events.items.length">{{ t('user_management.no_events') }}</p>
            <nav class="pagination" :aria-label="t('user_management.audit')"><Button :label="t('user_management.previous')" :disabled="loading || events.page <= 1" @click="eventPage(events.page - 1)" /><span>{{ t('user_management.page', { page: events.page, total: events.total }) }}</span><Button :label="t('user_management.next')" :disabled="loading || events.page * events.page_size >= events.total" @click="eventPage(events.page + 1)" /></nav>
        </section>
        <Dialog v-model:visible="createVisible" modal :header="t('user_management.create')" :style="{ width: '28rem', maxWidth: '95vw' }" :closable="!disabled" :close-on-escape="!disabled">
            <form class="dialog-form" @submit.prevent="create"><label for="new-user-name">{{ t('user_management.username') }}</label><InputText id="new-user-name" v-model="username" autofocus autocomplete="off" aria-describedby="username-help" :disabled="disabled" /><p id="username-help">{{ t('user_management.username_help') }}</p><p>{{ t('user_management.create_help') }}</p><p v-if="error" role="alert">{{ errorText }}</p><Button type="submit" :label="t('user_management.create')" :disabled="disabled || !usernameValid" /></form>
        </Dialog>
        <Dialog v-model:visible="confirmVisible" modal :header="t('user_management.confirm')" :style="{ width: '30rem', maxWidth: '95vw' }" :closable="!disabled" :close-on-escape="!disabled">
            <template v-if="pending"><p class="username"><strong>{{ pending.user.username }}</strong></p><p>{{ t(`user_management.confirm_${pending.action}`) }}</p><p v-if="pending.action === 'access'">{{ t(pending.user.web_login_enabled ? 'user_management.disable' : 'user_management.enable') }}</p><p v-if="pending.action === 'role'">{{ t(pending.user.role === 'admin' ? 'user_management.make_user' : 'user_management.make_admin') }}</p><p v-if="error" role="alert">{{ errorText }}</p><div class="actions"><Button :label="t('user_management.cancel')" :disabled="disabled" autofocus @click="pending = null" /><Button :label="t('user_management.confirm')" :disabled="disabled" @click="confirm" /></div></template>
        </Dialog>
        <Dialog v-model:visible="credentialVisible" modal :header="t('user_management.credential')" :style="{ width: '30rem', maxWidth: '95vw' }" @hide="revealed = false">
            <template v-if="credential"><p class="username">{{ credential.user.username }}</p><p>{{ t('user_management.credential_help') }}</p><label for="temporary-password">{{ t('user_management.credential') }}</label><InputText id="temporary-password" :type="revealed ? 'text' : 'password'" :value="credential.temporary_password" readonly autocomplete="off" /><p>{{ t('user_management.expires', { time: credential.expires_at }) }}</p><Button :label="t(revealed ? 'user_management.hide' : 'user_management.reveal')" @click="revealed = !revealed" /><Button :label="t('user_management.close')" @click="emit('dismiss-credential')" /></template>
        </Dialog>
    </section>
</template>

<style scoped>
.user-management { max-width: 100%; min-width: 0; padding: 1rem; }
header, .filters, .actions, .pagination { display: flex; gap: .75rem; align-items: center; flex-wrap: wrap; }
header { justify-content: space-between; }
.filters { margin: 1rem 0; align-items: end; }
.filters label, .dialog-form { display: flex; flex-direction: column; gap: .5rem; }
.filters input, select { max-width: 100%; padding: .5rem; }
.table-scroll { overflow-x: auto; max-width: 100%; }
table { width: 100%; min-width: 56rem; border-collapse: collapse; }
.table-scroll:focus-visible { outline: 2px solid var(--p-primary-color); outline-offset: 2px; }
th { white-space: nowrap; }
td.username { min-width: 12rem; }
th, td { padding: .65rem; text-align: start; border-bottom: 1px solid var(--p-content-border-color, #ddd); }
.username { overflow-wrap: anywhere; max-width: 22rem; }
.pagination { margin: 1rem 0; }
.details { margin-top: 1.5rem; border-top: 1px solid var(--p-content-border-color, #ddd); }
.audit { padding-inline-start: 1.2rem; overflow-wrap: anywhere; }
.sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0, 0, 0, 0); }
</style>
