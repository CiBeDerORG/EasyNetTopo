/** Controlled development DTO transport, never imported by production code. */
export function createCompositionFixture() {
    const calls: { method: string; networkId?: string }[] = [];
    const pending: { resolve: (value: unknown) => void; reject: (error: unknown) => void }[] = [];
    function request(method: string, networkId?: string) {
        calls.push({ method, networkId });
        return new Promise((resolve, reject) => pending.push({ resolve, reject }));
    }
    Object.assign(window, { compositionTransport: {
        calls,
        resolve(index: number, value: unknown) { pending[index].resolve(value); },
        reject(index: number) { pending[index].reject(new Error('Development failure')); },
    } });
    return { currentUserId: 42, api: {
        get_console_info: () => request('console'),
        list_machines: () => request('machines'),
        list_networks: () => request('networks'),
        list_network_members: (networkId: string) => request('members', networkId),
    } };
}
