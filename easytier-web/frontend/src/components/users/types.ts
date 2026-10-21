/** Component view contracts; transport ownership remains in the Web client. */
export interface UserSummary {
    id: number;
    username: string;
    role: 'admin' | 'user';
    web_login_enabled: boolean;
    source: 'unknown' | 'local' | 'oidc' | 'device';
    local_password_enabled: boolean | null;
    must_change_password: boolean;
    created_at: string | null;
    last_login_at: string | null;
}
export interface Page<T> { items: T[]; total: number; page: number; page_size: number }
export interface CurrentUser extends UserSummary { capabilities: string[]; default_password_warning: boolean }
export interface CredentialDelivery { user: UserSummary; temporary_password: string; expires_at: string }
export interface UserEvent {
    id: number; actor_id: number | null; target_id: number | null;
    action: string; occurred_at: string; result: 'success' | 'denied' | 'failure';
}
export interface UserListFilter {
    search?: string; role?: 'admin' | 'user'; web_login_enabled?: boolean;
    page: number; page_size: number;
}
