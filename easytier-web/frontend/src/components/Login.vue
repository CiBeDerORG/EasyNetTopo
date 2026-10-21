<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { Card, InputText, Password, Button } from 'primevue';
import { useRoute, useRouter } from 'vue-router';
import { I18nUtils } from 'easytier-frontend-lib';
import { getApiBase } from "../modules/api-host"
import { useI18n } from 'vue-i18n'
import ApiClient, { RegisterData } from '../modules/api';
import { toUserManagementError, type AuthPolicy } from '../modules/userManagement';

const { t } = useI18n()

defineProps<{
    isRegistering: boolean;
}>();

const api = computed<ApiClient>(() => new ApiClient(getApiBase()));
const router = useRouter();
const route = useRoute();
const policy = ref<AuthPolicy | null>(null);
const policyFailed = ref(false);
const busy = ref(false);
const errorCode = ref<string | null>(null);
async function loadPolicy() {
    policyFailed.value = false;
    try { policy.value = await api.value.userManagement.getPolicy(); } catch { policyFailed.value = true; }
}



const username = ref('');
const password = ref('');
const registerUsername = ref('');
const registerPassword = ref('');
const captcha = ref('');
const captchaSrc = computed(() => api.value.captcha_url());

const onSubmit = async () => {
    if (busy.value) return;
    busy.value = true; errorCode.value = null;
    try {
        const ret = await api.value.login({ username: username.value, password: password.value });
        if (!ret.success) { errorCode.value = ret.code ?? 'invalid_credentials'; return; }
        password.value = '';
        const identity = await api.value.userManagement.getMe();
        await router.push({ name: identity.must_change_password ? 'changePassword' : 'dashboard' });
    } catch (error) { errorCode.value = toUserManagementError(error).code; }
    finally { busy.value = false; }
};
const onRegister = async () => {
    if (busy.value || !policy.value?.registration_enabled) return;
    busy.value = true; errorCode.value = null;
    try {
        const request: RegisterData = { credentials: { username: registerUsername.value, password: registerPassword.value }, captcha: captcha.value };
        const ret = await api.value.register(request);
        if (ret.success) { registerPassword.value = ''; await router.push({ name: 'login' }); }
        else { errorCode.value = ret.code ?? 'invalid_request'; await loadPolicy(); }
    } catch (error) { errorCode.value = toUserManagementError(error).code; }
    finally { busy.value = false; }
};
const oidcEnabled = computed(() => policy.value?.oidc_enabled === true);
const onSsoLogin = () => {
    window.location.href = api.value.oidcLoginUrl();
};

onMounted(async () => {
    await loadPolicy();
});

</script>

<template>
    <div class="flex items-center justify-center min-h-screen">
        <Card class="w-full max-w-md p-6">
            <template #header>
                <h2 class="text-2xl font-semibold text-center">{{ isRegistering ? t('web.login.register') :
                    t('web.login.login') }}
                </h2>
            </template>
            <template #content>
                <p v-if="route.query.reason === 'expired'" role="status">{{ t('user_management.session_expired') }}</p>
                <p v-if="route.query.reason === 'password_changed'" role="status">{{ t('user_management.password_changed') }}</p>
                <p v-if="errorCode" role="alert">{{ t(`user_management.errors.${errorCode}`) }}</p>
                <div v-if="policyFailed" role="alert">{{ t('user_management.policy_failed') }} <Button :label="t('user_management.retry')" @click="loadPolicy" /></div>
                <p v-if="policy && !policy.registration_enabled">{{ t('user_management.registration_disabled') }}</p>
                <form v-if="!isRegistering" @submit.prevent="onSubmit" class="space-y-4">
                    <div class="p-field">
                        <label for="username" class="block text-sm font-medium">{{ t('web.login.username') }}</label>
                        <InputText id="username" v-model="username" required class="w-full" />
                    </div>
                    <div class="p-field">
                        <label for="password" class="block text-sm font-medium">{{ t('web.login.password') }}</label>
                        <Password input-id="password" v-model="password" required toggleMask :feedback="false" />
                    </div>
                    <div class="flex items-center justify-between">
                        <Button :label="t('web.login.login')" type="submit" class="w-full" :disabled="busy" :loading="busy" />
                    </div>
                    <div class="flex items-center justify-between">
                        <Button v-if="policy?.registration_enabled" :label="t('web.login.register')" type="button" class="w-full"
                            @click="$router.replace({ name: 'register' })" severity="secondary" />
                    </div>
                    <div v-if="oidcEnabled" class="flex items-center justify-between">
                        <Button :label="t('web.login.sso_login')" type="button" class="w-full" severity="info"
                            @click="onSsoLogin" />
                    </div>
                </form>

                <form v-else-if="policy?.registration_enabled" @submit.prevent="onRegister" class="space-y-4">
                    <div class="p-field">
                        <label for="register-username" class="block text-sm font-medium">{{ t('web.login.username')
                        }}</label>
                        <InputText id="register-username" v-model="registerUsername" required class="w-full" />
                    </div>
                    <div class="p-field">
                        <label for="register-password" class="block text-sm font-medium">{{ t('web.login.password')
                        }}</label>
                        <Password input-id="register-password" v-model="registerPassword" required toggleMask
                            :feedback="false" />
                    </div>
                    <div class="p-field">
                        <label for="captcha" class="block text-sm font-medium">{{ t('web.login.captcha') }}</label>
                        <InputText id="captcha" v-model="captcha" required class="w-full" />
                        <img :src="captchaSrc" alt="Captcha" class="mt-2 mb-2" />
                    </div>
                    <div class="flex items-center justify-between">
                        <Button :label="t('web.login.register')" type="submit" class="w-full" :disabled="busy" :loading="busy" />
                    </div>
                    <div class="flex items-center justify-between">
                        <Button :label="t('web.login.back_to_login')" type="button" class="w-full"
                            @click="$router.replace({ name: 'login' })" severity="secondary" />
                    </div>
                </form>

                <Button v-if="isRegistering && !policy?.registration_enabled" :label="t('web.login.back_to_login')" @click="router.replace({ name: 'login' })" />
                <Button icon="pi pi-language" type="button" class="rounded-full absolute top-4 right-4 z-10"
                    style="box-shadow: 0 2px 8px rgba(0,0,0,0.08);" severity="contrast"
                    @click="I18nUtils.toggleLanguage" :aria-label="t('web.main.language')"
                    :v-tooltip="t('web.main.language')" />

            </template>


        </Card>
    </div>
</template>

<style scoped></style>
