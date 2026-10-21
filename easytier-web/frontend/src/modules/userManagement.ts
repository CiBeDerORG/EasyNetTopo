import axios, { type AxiosInstance, type AxiosRequestConfig } from 'axios';

// WUM contract v1. Wire names and unknown historical values are preserved.
export type UserRole = 'admin' | 'user';
export interface UserSummary {
    id: number;
    username: string;
    role: UserRole;
    web_login_enabled: boolean;
    source: 'unknown' | 'local' | 'oidc' | 'device';
    local_password_enabled: boolean | null;
    must_change_password: boolean;
    created_at: string | null;
    last_login_at: string | null;
}
export interface Page<T> { items: T[]; total: number; page: number; page_size: number }
export interface CurrentUser extends UserSummary { capabilities: string[]; default_password_warning: boolean }
export interface AuthPolicy { registration_enabled: boolean; oidc_enabled: boolean }
export interface CredentialDelivery { user: UserSummary; temporary_password: string; expires_at: string }
export interface UserEvent {
    id: number;
    actor_id: number | null;
    target_id: number | null;
    action: 'create' | 'access' | 'role' | 'password_reset' | 'revoke_sessions' | 'bootstrap' | 'recover' | 'identity_bind';
    occurred_at: string;
    result: 'success' | 'denied' | 'failure';
}
export interface UserListFilter {
    search?: string;
    role?: UserRole;
    web_login_enabled?: boolean;
    page?: number;
    page_size?: number;
}
export interface UserEventFilter { target_id?: number; page?: number; page_size?: number }

const errorMessages: Record<string, string> = {
    invalid_request: 'Invalid request.',
    unauthenticated: 'Please sign in again.',
    invalid_credentials: 'Invalid credentials.',
    forbidden: 'You do not have permission for this action.',
    password_change_required: 'Change your password before continuing.',
    self_action_forbidden: 'This action cannot target your own account.',
    local_password_unavailable: 'Local password reset is unavailable for this account.',
    csrf_rejected: 'The request origin could not be verified.',
    user_not_found: 'The account could not be found.',
    username_conflict: 'The username is already in use.',
    last_admin: 'At least one enabled administrator is required.',
    identity_conflict: 'The external identity requires an explicit account binding.',
    internal_error: 'The operation could not be completed.',
    network_error: 'The server could not be reached.',
    request_failed: 'The request could not be completed.',
};

/** Safe to render: never retains an Axios request, response, or arbitrary server body. */
export class UserManagementError extends Error {
    readonly status: number | null;
    readonly code: string;

    constructor(status: number | null, code: string) {
        super(errorMessages[code] ?? errorMessages.request_failed);
        this.name = 'UserManagementError';
        this.status = status;
        this.code = code;
    }
}

export function toUserManagementError(error: unknown): UserManagementError {
    if (error instanceof UserManagementError) return error;
    if (!axios.isAxiosError(error)) return new UserManagementError(null, 'request_failed');
    const status = error.response?.status ?? null;
    const body: unknown = error.response?.data;
    const code = typeof body === 'object' && body !== null && 'code' in body ? body.code : undefined;
    if (typeof code === 'string' && Object.prototype.hasOwnProperty.call(errorMessages, code)) {
        return new UserManagementError(status, code);
    }
    const fallback = status === null ? 'network_error'
        : status === 401 ? 'unauthenticated'
        : status === 403 ? 'forbidden'
        : status === 400 ? 'invalid_request'
        : status >= 500 ? 'internal_error' : 'request_failed';
    return new UserManagementError(status, fallback);
}

/** Uses ApiClient's session-aware, data-unwrapping transport. No retries or credential cache. */
export class UserManagementClient {
    constructor(private readonly client: AxiosInstance) {}

    private async request<T>(config: AxiosRequestConfig): Promise<T> {
        try {
            return await this.client.request<unknown, T>(config);
        } catch (error: unknown) {
            throw toUserManagementError(error);
        }
    }

    private write<T>(method: 'post' | 'put', url: string, data: unknown): Promise<T> {
        return this.request<T>({ method, url, data, headers: {
            'Content-Type': 'application/json', 'X-Requested-With': 'EasyTier',
        } });
    }

    getMe(): Promise<CurrentUser> { return this.request({ method: 'get', url: '/auth/me' }); }
    getPolicy(): Promise<AuthPolicy> { return this.request({ method: 'get', url: '/auth/policy' }); }
    listUsers(filter: UserListFilter = {}): Promise<Page<UserSummary>> {
        return this.request({ method: 'get', url: '/admin/users', params: { page: 1, page_size: 20, ...filter } });
    }
    getUser(id: number): Promise<UserSummary> { return this.request({ method: 'get', url: `/admin/users/${id}` }); }
    createUser(username: string): Promise<CredentialDelivery> { return this.write('post', '/admin/users', { username }); }
    setAccess(id: number, enabled: boolean): Promise<UserSummary> {
        return this.write('put', `/admin/users/${id}/access`, { web_login_enabled: enabled });
    }
    setRole(id: number, role: UserRole): Promise<UserSummary> {
        return this.write('put', `/admin/users/${id}/role`, { role });
    }
    resetPassword(id: number): Promise<CredentialDelivery> { return this.write('post', `/admin/users/${id}/password-reset`, {}); }
    revokeSessions(id: number): Promise<UserSummary> { return this.write('post', `/admin/users/${id}/revoke-sessions`, {}); }
    listEvents(filter: UserEventFilter = {}): Promise<Page<UserEvent>> {
        return this.request({ method: 'get', url: '/admin/user-events', params: { page: 1, page_size: 20, ...filter } });
    }
}
