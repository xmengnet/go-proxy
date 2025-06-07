const { createApp, ref, computed, onMounted } = Vue;
createApp({
    setup() {
        const apiKeysInput = ref('');
        const results = ref([]);
        const error = ref('');
        const loading = ref(false);
        const isDark = ref(false);
        const copied = ref(false);

        // 计算有效的API Keys
        const validApiKeys = computed(() => {
            return results.value
                .filter(r => !r.error)
                .map(r => r.apiKey);
        });

        // 复制有效的API Keys
        async function copyValidKeys() {
            if (!validApiKeys.value.length) return;
            try {
                await navigator.clipboard.writeText(validApiKeys.value.join(','));
                copied.value = true;
                setTimeout(() => {
                    copied.value = false;
                }, 2000);
            } catch (err) {
                console.error('复制失败:', err);
            }
        }

        function maskKey(key) {
            if (!key) return '';
            if (key.length <= 8) return key;
            return key.slice(0, 4) + '****' + key.slice(-4);
        }

        const updateHtmlClass = (darkMode) => {
            if (darkMode) {
                document.documentElement.classList.add('dark');
            } else {
                document.documentElement.classList.remove('dark');
            }
        };

        const toggleDarkMode = () => {
            isDark.value = !isDark.value;
            localStorage.setItem('darkMode', String(isDark.value));
            updateHtmlClass(isDark.value);
        };

        async function fetchBalances() {
            error.value = '';
            results.value = [];
            loading.value = true;
            let keys = apiKeysInput.value.split(',').map(k => k.trim()).filter(Boolean);
            if (!keys.length) {
                error.value = '请输入至少一个 API Key';
                loading.value = false;
                return;
            }
            const promises = keys.map(async (apiKey) => {
                try {
                    const res = await fetch('https://api.siliconflow.cn/v1/user/info', {
                        headers: { 'Authorization': 'Bearer ' + apiKey }
                    });
                    if (!res.ok) throw new Error('请求失败，状态码：' + res.status);
                    const data = await res.json();
                    if (data && data.data) {
                        return { apiKey, balance: data.data.balance, userInfo: data.data, error: null };
                    } else {
                        throw new Error('返回数据格式异常');
                    }
                } catch (e) {
                    return { apiKey, balance: null, userInfo: null, error: e.message || '查询失败' };
                }
            });
            results.value = await Promise.all(promises);
            loading.value = false;
        }

        onMounted(() => {
            const storedDarkMode = localStorage.getItem('darkMode');
            if (storedDarkMode !== null) {
                isDark.value = storedDarkMode === 'true';
            } else {
                isDark.value = document.documentElement.classList.contains('dark');
            }
            updateHtmlClass(isDark.value);
        });

        return {
            apiKeysInput,
            results,
            error,
            loading,
            isDark,
            copied,
            validApiKeys,
            fetchBalances,
            maskKey,
            toggleDarkMode,
            copyValidKeys
        };
    }
}).mount('#app');
