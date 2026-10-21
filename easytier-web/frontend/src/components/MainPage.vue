<script setup lang="ts">
import { applyThemeMode, storedThemeMode, type ThemeMode } from '../modules/theme';

const themeMode = ref<ThemeMode>(storedThemeMode());
const themeCycle = [
    { mode: 'system' as ThemeMode, icon: 'pi pi-desktop' },
    { mode: 'light' as ThemeMode, icon: 'pi pi-sun' },
    { mode: 'dark' as ThemeMode, icon: 'pi pi-moon' },
];
const themeIcon = computed(() => themeCycle.find(entry => entry.mode === themeMode.value)?.icon ?? 'pi pi-desktop');
const cycleThemeMode = () => {
    const index = themeCycle.findIndex(entry => entry.mode === themeMode.value);
    const next = themeCycle[(index + 1) % themeCycle.length].mode;
    themeMode.value = next;
    applyThemeMode(next);
};

import { I18nUtils } from 'easytier-frontend-lib'
import { computed, onMounted, onUnmounted, nextTick, ref, watch, type ComponentPublicInstance } from 'vue';
import { Button, Drawer, Menu, OverlayBadge, Popover, TieredMenu, useToast } from 'primevue';
import { useRoute, useRouter } from 'vue-router';
import ChangePassword from './ChangePassword.vue';
import UserManagement from './users/UserManagement.vue';
import { toUserManagementError, type CurrentUser, type UserSummary, type UserEvent, type Page, type UserListFilter, type UserEventFilter, type CredentialDelivery } from '../modules/userManagement';
import Icon from '../assets/easytier.png'
import { useI18n } from 'vue-i18n'
import ApiClient, { type BlockedDevice, type CentralNetworkSummary } from '../modules/api';
import { getApiBase } from '../modules/api-host';

const { t } = useI18n()
const router = useRouter();
const toast = useToast();
const route = useRoute();
const api = new ApiClient(getApiBase(), expireSession);

const userMenu = ref();
const userMenuItems = computed(() => [
    {
        label: t('web.main.change_password'),
        icon: 'pi pi-key',
        visible: currentUser.value?.local_password_enabled === true,
        command: () => router.push({ name: 'changePassword' }),
    },
    {
        label: t('web.main.logout'),
        icon: 'pi pi-sign-out',
        command: async () => {
            try {
                await api.logout();
            } catch (e) {
                // Local state is cleared even if the server is unreachable.
            }
            expireSession();
        },
    },
]);

// Identity is owned here; child views receive server state rather than infer roles.
const currentUser = ref<CurrentUser | null>(null);
const identityError = ref<string | null>(null);
const canManage = computed(() => !!currentUser.value?.capabilities.includes('users.manage') && !currentUser.value.must_change_password);
const users = ref<Page<UserSummary>>({ items: [], total: 0, page: 1, page_size: 20 });
const events = ref<Page<UserEvent>>({ items: [], total: 0, page: 1, page_size: 20 });
const selectedUser = ref<UserSummary | null>(null);
const credential = ref<CredentialDelivery | null>(null);
const managementError = ref<{ code: string; message: string } | null>(null);
const loading = ref(false);
const busy = ref(false);
const stale = ref(false);
let userQuery: UserListFilter = { page: 1, page_size: 20 };
let listSequence = 0, detailSequence = 0, eventSequence = 0, identitySequence = 0, mutationSequence = 0;
function disposeRequests() { listSequence++; detailSequence++; eventSequence++; identitySequence++; mutationSequence++; credential.value = null; }
function expireSession() {
    disposeRequests(); currentUser.value = null;
    void router.replace({ name: 'login', query: { reason: 'expired' } });
}
async function refreshIdentity() {
    const sequence = ++identitySequence;
    try {
        const identity = await api.userManagement.getMe();
        if (sequence !== identitySequence) return false;
        currentUser.value = identity; identityError.value = null;
        if (identity.must_change_password) { credential.value = null; await router.replace({ name: 'changePassword' }); }
        if (!canManage.value) { credential.value = null; listSequence++; detailSequence++; eventSequence++; }
        return true;
    } catch (error) {
        if (sequence !== identitySequence) return false;
        const safe = toUserManagementError(error);
        identityError.value = safe.status === 403 ? 'forbidden' : safe.code;
        if (safe.status === 401) expireSession();
        return false;
    }
}
async function managementFailure(error: unknown) {
    const safe = toUserManagementError(error);
    managementError.value = safe;
    if (safe.status === 401) expireSession();
    else if (safe.status === 403) await refreshIdentity();
}
async function loadUsers(filter: UserListFilter = userQuery) {
    if (!canManage.value) return;
    userQuery = { ...filter };
    const sequence = ++listSequence;
    loading.value = true; managementError.value = null;
    try {
        const result = await api.userManagement.listUsers(filter);
        if (sequence !== listSequence || route.name !== 'users') return;
        users.value = result; stale.value = false;
    } catch (error) {
        if (sequence === listSequence) { stale.value = users.value.items.length > 0; await managementFailure(error); }
    } finally { if (sequence === listSequence) loading.value = false; }
}
async function selectUser(id: number) {
    const sequence = ++detailSequence;
    selectedUser.value = null;
    events.value = { items: [], total: 0, page: 1, page_size: 20 };
    try { const result = await api.userManagement.getUser(id); if (sequence === detailSequence && canManage.value && route.name === 'users') selectedUser.value = result; }
    catch (error) { if (sequence === detailSequence) await managementFailure(error); }
}
async function loadEvents(filter: UserEventFilter) {
    const sequence = ++eventSequence;
    try { const result = await api.userManagement.listEvents(filter); if (sequence === eventSequence && canManage.value && route.name === 'users') events.value = result; }
    catch (error) { if (sequence === eventSequence) await managementFailure(error); }
}
async function mutate(action: () => Promise<UserSummary | CredentialDelivery>, selfRevoke = false) {
    if (busy.value || !canManage.value) return;
    const sequence = ++mutationSequence;
    busy.value = true; managementError.value = null; credential.value = null;
    try {
        const result = await action();
        if (sequence !== mutationSequence || route.name !== 'users') return;
        if (selfRevoke) { expireSession(); return; }
        if ('temporary_password' in result) credential.value = result;
        if (selectedUser.value) await selectUser(selectedUser.value.id);
        await Promise.all([loadUsers(), refreshIdentity()]);
        if (selectedUser.value) await loadEvents({ target_id: selectedUser.value.id, page: 1, page_size: 20 });
    } catch (error) { if (sequence === mutationSequence) await managementFailure(error); }
    finally { if (sequence === mutationSequence) busy.value = false; }
}
watch([() => route.name, canManage], ([name, allowed]) => {
    credential.value = null;
    if (name === 'users' && allowed) void loadUsers();
    else { listSequence++; detailSequence++; eventSequence++; mutationSequence++; busy.value = false; loading.value = false; }
});

// Networks are first-class navigation targets: each network gets a direct
// entry so managing one is a single click from anywhere.
const networks = ref<CentralNetworkSummary[]>([]);
// Blocked devices that still try to come online raise the bell badge.
const blockedDevices = ref<BlockedDevice[]>([]);
const loadNetworks = async () => {
    if (!centralEnabled.value) return;
    try {
        networks.value = (await api.list_networks()) ?? [];
    } catch (e) {
        // Keep the last known list; the workspace links stay usable.
    }
};
const loadBlockedDevices = async () => {
    if (!centralEnabled.value) return;
    try {
        blockedDevices.value = (await api.list_blocked_devices()) ?? [];
    } catch (e) {
        // Notifications are best-effort.
    }
};
let networkTimer: number | undefined;
const loadNavigation = async () => {
    if (!await refreshIdentity() || currentUser.value?.must_change_password) return;
    if (!enrollInfo.value) {
        try {
            enrollInfo.value = await api.get_console_info();
        } catch (error) {
            console.error('Failed to load console mode', error);
            return;
        }
    }
    await Promise.all([loadNetworks(), loadBlockedDevices()]);
};
onMounted(() => {
    void loadNavigation();
    networkTimer = window.setInterval(loadNavigation, 10_000);
});
onUnmounted(() => { window.clearInterval(networkTimer); disposeRequests(); });

// Device enrollment info: the config-server command devices run to join
// this console. Also identifies whether central management is available.
const enrollBell = ref();
const enrollInfo = ref<import('../modules/api').ConsoleInfo | null>(null);
const enrollLoading = ref(false);
const centralEnabled = computed(() => enrollInfo.value?.webhook_auth === false);
watch([() => enrollInfo.value?.webhook_auth, () => route.name], ([external, name]) => {
    if (external && (name === 'networkList' || name === 'networkDetail')) {
        router.replace({ name: 'dashboard' });
    }
});

const enrollCommand = computed(() => {
    if (!enrollInfo.value) return '';
    const host = new URL(getApiBase(), window.location.href).hostname;
    const { config_server_protocol: proto, config_server_port: port, username } = enrollInfo.value;
    return `easytier-core --config-server ${proto}://${host}:${port}/${username}`;
});

const toggleEnrollBell = async (event: Event) => {
    enrollBell.value.toggle(event);
    if (!enrollInfo.value && !enrollLoading.value) {
        enrollLoading.value = true;
        try {
            enrollInfo.value = await api.get_console_info();
        } catch (e) {
            console.error(e);
        } finally {
            enrollLoading.value = false;
        }
    }
};

const copyEnrollCommand = async () => {
    try {
        await navigator.clipboard.writeText(enrollCommand.value);
        toast.add({ severity: 'success', summary: t('web.common.confirm'), life: 1500 });
    } catch (e) {
        console.error(e);
    }
};

const blockedAttemptCount = computed(() =>
    blockedDevices.value.filter(device => device.attempt_count > 0).length);
const blockBell = ref();
const toggleBlockBell = (event: Event) => blockBell.value.toggle(event);
const unblockDevice = async (device: BlockedDevice) => {
    try {
        await api.unblock_device(device.id);
        await loadBlockedDevices();
    } catch (e: any) {
        toast.add({ severity: 'error', summary: t('web.console.blocked_devices'), detail: e?.response?.data?.message ?? String(e), life: 5000 });
    }
};

type NavItem = {
    key: string;
    label: string;
    icon: string;
    to: { name: string; params?: Record<string, string> };
    badge?: string;
    child?: boolean;
};

const workspaceNavigation = computed<NavItem[]>(() => [
    { key: 'dashboard', label: t('web.main.dashboard'), icon: 'pi pi-chart-pie', to: { name: 'dashboard' } },
    { key: 'deviceList', label: t('web.main.device_list'), icon: 'pi pi-server', to: { name: 'deviceList' } },
    { key: 'ecosystem', label: t('web.ecosystem.title'), icon: 'pi pi-sitemap', to: { name: 'ecosystem' } },
    ...(canManage.value ? [{ key: 'users', label: t('user_management.title'), icon: 'pi pi-users', to: { name: 'users' } }] : []),
]);
const networkNavigation = computed<NavItem[]>(() => [
    { key: 'networkList', label: t('web.main.network_list'), icon: 'pi pi-globe', to: { name: 'networkList' } },
    ...networks.value.map(network => ({
        key: `network:${network.network_id}`,
        label: network.display_name || network.network_name,
        icon: 'pi pi-sitemap',
        to: { name: 'networkDetail', params: { networkId: network.network_id } },
        badge: `${network.online_member_count}/${network.member_count}`,
        child: true,
    })),
]);

const activeKey = computed(() => {
    if (route.name === 'networkDetail') return `network:${route.params.networkId}`;
    if (route.name === 'deviceManagement') return 'deviceList';
    if (typeof route.name === 'string') return route.name;
    return '';
});
const pageTitle = computed(() => {
    if (route.name === 'networkDetail') {
        const network = networks.value.find(item => `network:${item.network_id}` === activeKey.value);
        if (network) return network.display_name || network.network_name;
        return t('web.main.network_list');
    }
    return [...workspaceNavigation.value, ...networkNavigation.value]
        .find(item => item.key === activeKey.value)?.label;
});
const forceShowSideBar = ref(false);
const mainContent = ref<HTMLElement>();
const mobileNavigationToggle = ref<ComponentPublicInstance>();
let navigationOriginPath: string | undefined;

function openMobileNavigation() {
    navigationOriginPath = route.fullPath;
    forceShowSideBar.value = true;
}

async function restoreNavigationFocus() {
    if (navigationOriginPath === undefined) return;
    const navigated = navigationOriginPath !== route.fullPath;
    navigationOriginPath = undefined;
    await nextTick();
    const toggle = mobileNavigationToggle.value?.$el as HTMLElement | undefined;
    const target = !navigated && toggle?.isConnected && toggle.getClientRects().length
        ? toggle : mainContent.value;
    target?.focus();
}

watch(() => currentUser.value?.must_change_password, async required => {
    if (!required || navigationOriginPath === undefined) return;
    forceShowSideBar.value = false;
    await restoreNavigationFocus();
});

// 切换页面后关闭移动端侧边栏。
watch(() => route.fullPath, () => { forceShowSideBar.value = false; });
</script>

<template>
    <div class="web-console">
        <a class="skip-link" href="#main-content" @click.prevent="mainContent?.focus()">{{ t('web.console.skip_content') }}</a>
        <aside v-if="!currentUser?.must_change_password" class="console-sidebar">
            <RouterLink :to="{ name: 'dashboard' }" class="console-brand">
                <img :src="Icon" alt="" /><span>EasyTier<span class="brand-caption">{{ t('web.console.console') }}</span></span>
            </RouterLink>
            <div class="nav-caption">{{ t('web.console.workspace') }}</div>
            <nav :aria-label="t('web.console.navigation')">
                <Menu :model="workspaceNavigation" :aria-label="t('web.console.navigation')" class="console-navigation">
                    <template #item="{ item, props }">
                        <RouterLink :to="item.to" custom v-slot="{ href, navigate }">
                            <a v-bind="props.action" :href="href" @click="navigate"
                                :class="{ 'is-active': activeKey === item.key }" :aria-current="activeKey === item.key ? 'page' : undefined">
                                <span v-bind="props.icon" aria-hidden="true"></span><span v-bind="props.label">{{ item.label }}</span>
                            </a>
                        </RouterLink>
                    </template>
                </Menu>
                <div v-if="centralEnabled" class="nav-caption">{{ t('web.main.network_list') }}</div>
                    <Menu v-if="centralEnabled" :model="networkNavigation" :aria-label="t('web.main.network_list')" class="console-navigation">
                        <template #item="{ item, props }">
                            <RouterLink :to="item.to" custom v-slot="{ href, navigate }">
                                <a v-bind="props.action" :href="href" @click="navigate"
                                    :class="{ 'is-active': activeKey === item.key, 'nav-child': item.child }" :aria-current="activeKey === item.key ? 'page' : undefined">
                                    <span v-bind="props.icon" aria-hidden="true"></span><span v-bind="props.label">{{ item.label }}</span>
                                    <span v-if="item.badge" class="nav-badge">{{ item.badge }}</span>
                                </a>
                            </RouterLink>
                        </template>
                </Menu>
            </nav>
            <a href="https://easytier.cn" target="_blank" rel="noopener noreferrer" class="console-docs">
                <i class="pi pi-book" aria-hidden="true"></i>{{ t('web.console.documentation') }}<i class="pi pi-arrow-up-right" aria-hidden="true"></i>
            </a>
        </aside>
        <Drawer v-if="!currentUser?.must_change_password" v-model:visible="forceShowSideBar" :header="t('web.console.navigation')" class="console-mobile-nav" @after-hide="restoreNavigationFocus">
            <nav :aria-label="t('web.console.navigation')">
                <div class="nav-caption">{{ t('web.console.workspace') }}</div>
                <Menu :model="workspaceNavigation" :aria-label="t('web.console.navigation')" class="console-navigation">
                    <template #item="{ item, props }">
                        <RouterLink :to="item.to" custom v-slot="{ href, navigate }">
                            <a v-bind="props.action" :href="href" @click="navigate($event); forceShowSideBar = false"
                                :class="{ 'is-active': activeKey === item.key }" :aria-current="activeKey === item.key ? 'page' : undefined">
                                <span v-bind="props.icon" aria-hidden="true"></span><span v-bind="props.label">{{ item.label }}</span>
                            </a>
                        </RouterLink>
                    </template>
                </Menu>
                <div v-if="centralEnabled" class="nav-caption">{{ t('web.main.network_list') }}</div>
                    <Menu v-if="centralEnabled" :model="networkNavigation" :aria-label="t('web.main.network_list')" class="console-navigation">
                        <template #item="{ item, props }">
                            <RouterLink :to="item.to" custom v-slot="{ href, navigate }">
                                <a v-bind="props.action" :href="href" @click="navigate($event); forceShowSideBar = false"
                                    :class="{ 'is-active': activeKey === item.key, 'nav-child': item.child }" :aria-current="activeKey === item.key ? 'page' : undefined">
                                    <span v-bind="props.icon" aria-hidden="true"></span><span v-bind="props.label">{{ item.label }}</span>
                                    <span v-if="item.badge" class="nav-badge">{{ item.badge }}</span>
                                </a>
                            </RouterLink>
                        </template>
                </Menu>
            </nav>
        </Drawer>
        <div class="console-workspace">
            <header class="console-topbar">
                <div class="console-breadcrumb flex items-center gap-3 min-w-0">
                    <Button v-if="!currentUser?.must_change_password" ref="mobileNavigationToggle" icon="pi pi-bars" text severity="secondary" class="mobile-nav-toggle"
                        :aria-label="t('web.console.navigation')" @click="openMobileNavigation" />
                    <span class="topbar-brand">{{ t('web.console.workspace') }}</span>
                    <i class="pi pi-angle-right topbar-brand" aria-hidden="true"></i><span class="console-page-title" :title="pageTitle">{{ pageTitle }}</span>
                </div>
                <div class="console-topbar-actions flex items-center gap-2">
                    <Button v-if="currentUser && !currentUser.must_change_password" icon="pi pi-objects-column" :label="t('web.console.enroll_title')"
                        severity="secondary" outlined size="small" class="enroll-button"
                        :aria-label="t('web.console.enroll_title')"
                        @click="toggleEnrollBell" />
                    <Popover ref="enrollBell" appendTo="body" style="min-width: 34rem">
                        <div class="flex flex-col gap-2">
                            <span class="font-semibold">{{ t('web.console.enroll_title') }}</span>
                            <p class="text-sm muted m-0">{{ t('web.console.enroll_hint') }}</p>
                            <div v-if="enrollLoading" class="text-sm muted">{{ t('web.common.loading') }}</div>
                            <template v-else-if="enrollInfo">
                                <div class="flex items-center gap-2">
                                    <code class="mono-value flex-1 p-2 rounded"
                                        style="background: var(--console-ground); word-break: break-all">{{ enrollCommand }}</code>
                                    <Button icon="pi pi-copy" severity="secondary" outlined size="small"
                                        :aria-label="t('web.console.enroll_copy')" @click="copyEnrollCommand" />
                                </div>
                                <p class="text-xs muted m-0">
                                    {{ enrollInfo.webhook_auth ? t('web.console.enroll_webhook_note') : t('web.console.enroll_token_note') }}
                                </p>
                            </template>
                        </div>
                    </Popover>
                    <OverlayBadge v-if="centralEnabled" :value="blockedAttemptCount > 0 ? String(blockedAttemptCount) : undefined">
                        <Button icon="pi pi-bell" text severity="secondary" :aria-label="t('web.console.blocked_devices')" @click="toggleBlockBell" />
                    </OverlayBadge>
                    <Popover ref="blockBell" appendTo="body" style="min-width: 22rem">
                        <div class="flex items-center justify-between mb-2">
                            <span class="font-semibold">{{ t('web.console.blocked_devices') }}</span>
                        </div>
                        <div v-if="blockedDevices.length === 0" class="text-sm muted p-2">{{ t('web.console.blocked_empty') }}</div>
                        <div v-else class="flex flex-col gap-2">
                            <div v-for="device in blockedDevices" :key="device.id" class="flex items-center justify-between gap-3 border-b border-surface last:border-b-0 pb-2 last:pb-0">
                                <div class="min-w-0">
                                    <div class="font-medium truncate">{{ device.alias || device.hostname || device.id }}</div>
                                    <div class="text-xs muted">
                                        <template v-if="device.attempt_count > 0">
                                            {{ t('web.console.blocked_attempted', { time: (device.last_attempt_time || '').replace('T', ' ').slice(0, 19), count: device.attempt_count }) }}
                                        </template>
                                        <template v-else>{{ t('web.console.blocked_no_attempt') }}</template>
                                    </div>
                                </div>
                                <Button :label="t('web.console.unblock')" size="small" severity="secondary" outlined @click="unblockDevice(device)" />
                            </div>
                        </div>
                    </Popover>
                    <Button :icon="themeIcon" text severity="secondary"
                        :aria-label="t('web.console.theme_mode')" v-tooltip.bottom="t('web.console.theme_mode')"
                        @click="cycleThemeMode" />
                    <Button icon="pi pi-language" text severity="secondary" :aria-label="t('web.console.language')" @click="I18nUtils.toggleLanguage" />
                    <span class="topbar-divider"></span>
                    <Button type="button" @click="userMenu.toggle($event)" aria-haspopup="true" aria-controls="user-menu"
                        :aria-label="t('web.console.account')" icon="pi pi-user" text severity="secondary" />
                    <TieredMenu ref="userMenu" id="user-menu" :model="userMenuItems" popup />
                </div>
            </header>
            <main ref="mainContent" id="main-content" class="console-content" tabindex="-1">
                <div v-if="identityError" role="alert">{{ t(`user_management.errors.${identityError}`) }} <Button :label="t('user_management.retry')" @click="loadNavigation" /></div>
                <div v-if="currentUser?.default_password_warning" role="alert" class="mb-4">
                    {{ t('user_management.default_password_warning') }} <Button v-if="currentUser.local_password_enabled === true" :label="t('web.main.change_password')" @click="router.push({ name: 'changePassword' })" />
                </div>
                <p v-if="!currentUser && !identityError" role="status">{{ t('web.common.loading') }}</p>
                <template v-if="currentUser">
                <ChangePassword v-if="currentUser.must_change_password || route.name === 'changePassword'" :api="api" :required="currentUser.must_change_password" :local-password-enabled="currentUser.local_password_enabled" :source="currentUser.source" />
                <template v-else-if="route.name === 'users'">
                    <UserManagement v-if="canManage" :current-user="currentUser" :users="users" :events="events" :selected-user="selectedUser" :loading="loading" :busy="busy" :stale="stale" :error="managementError" :credential="credential"
                        @query="loadUsers" @select="selectUser" @query-events="loadEvents" @retry="loadUsers()" @dismiss-credential="credential = null"
                        @create="mutate(() => api.userManagement.createUser($event.username))"
                        @set-access="mutate(() => api.userManagement.setAccess($event.id, $event.web_login_enabled))"
                        @set-role="mutate(() => api.userManagement.setRole($event.id, $event.role))"
                        @reset-password="mutate(() => api.userManagement.resetPassword($event))"
                        @revoke-sessions="mutate(() => api.userManagement.revokeSessions($event), $event === currentUser.id)" />
                    <p v-else role="alert">{{ t('user_management.errors.forbidden') }}</p>
                </template>
                <RouterView v-else v-slot="{ Component }">
                    <component :is="Component" :api="api" :central-enabled="centralEnabled" :current-user-id="currentUser.id" :key="route.name === 'networkDetail' ? String(route.params.networkId) : undefined" />
                </RouterView>
                </template>
            </main>
        </div>
    </div>
</template>
