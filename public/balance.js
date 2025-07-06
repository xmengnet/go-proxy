const { createApp, ref, computed, onMounted } = Vue;
createApp({
    setup() {
        const apiKeysInput = ref('');
        const results = ref([]);
        const error = ref('');
        const loading = ref(false);
        const isDark = ref(false);
        const copied = ref(false);

        // 计算有效的API Keys（余额大于0的）
        const validApiKeys = computed(() => {
            return results.value
                .filter(r => !r.error && r.balance > 0)
                .map(r => r.apiKey);
        });

        // 计算有余额的API Keys数量
        const validWithBalanceCount = computed(() => {
            return results.value.filter(r => !r.error && r.balance > 0).length;
        });

        // 计算无余额的API Keys数量
        const validNoBalanceCount = computed(() => {
            return results.value.filter(r => !r.error && r.balance === 0).length;
        });

        // 计算无效的API Keys数量
        const invalidCount = computed(() => {
            return results.value.filter(r => r.error).length;
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
                // 降级到传统复制方法
                fallbackCopy(validApiKeys.value.join(','));
            }
        }

        // 复制所有有效的API Keys（包括无余额的）
        async function copyAllValidKeys() {
            const allValidKeys = results.value
                .filter(r => !r.error)
                .map(r => r.apiKey);
            if (!allValidKeys.length) return;
            try {
                await navigator.clipboard.writeText(allValidKeys.join(','));
                copied.value = true;
                setTimeout(() => {
                    copied.value = false;
                }, 2000);
            } catch (err) {
                console.error('复制失败:', err);
                fallbackCopy(allValidKeys.join(','));
            }
        }

        // 复制单个API Key
        async function copySingleKey(apiKey) {
            try {
                await navigator.clipboard.writeText(apiKey);
                copied.value = true;
                setTimeout(() => {
                    copied.value = false;
                }, 2000);
            } catch (err) {
                console.error('复制失败:', err);
                fallbackCopy(apiKey);
            }
        }

        // 降级复制方法
        function fallbackCopy(text) {
            const textArea = document.createElement('textarea');
            textArea.value = text;
            textArea.style.position = 'fixed';
            textArea.style.left = '-999999px';
            textArea.style.top = '-999999px';
            document.body.appendChild(textArea);
            textArea.focus();
            textArea.select();
            try {
                document.execCommand('copy');
                copied.value = true;
                setTimeout(() => {
                    copied.value = false;
                }, 2000);
            } catch (err) {
                console.error('降级复制也失败了:', err);
                alert('复制失败，请手动复制');
            }
            document.body.removeChild(textArea);
        }

        function maskKey(key) {
            if (!key) return '';
            if (key.length <= 8) return key;
            return key.slice(0, 4) + '****' + key.slice(-4);
        }

        // 获取状态文本和样式
        function getStatusInfo(result) {
            if (result.error) {
                return {
                    text: '无效',
                    class: 'status-invalid',
                    bgClass: 'bg-status-invalid'
                };
            } else if (result.balance > 0) {
                return {
                    text: '有效（有余额）',
                    class: 'status-valid-with-balance',
                    bgClass: 'bg-status-valid-with-balance'
                };
            } else {
                return {
                    text: '有效（无余额）',
                    class: 'status-valid-no-balance',
                    bgClass: 'bg-status-valid-no-balance'
                };
            }
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
            // 支持换行符和逗号分隔，先按换行符分割，再按逗号分割
            let keys = apiKeysInput.value
                .split(/[\n,]/) // 同时支持换行符和逗号分隔
                .map(k => k.trim()) // 去除首尾空格
                .filter(Boolean); // 过滤空字符串
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
            validWithBalanceCount,
            validNoBalanceCount,
            invalidCount,
            fetchBalances,
            maskKey,
            toggleDarkMode,
            copyValidKeys,
            copyAllValidKeys,
            copySingleKey,
            getStatusInfo
        };
    }
}).mount('#app');
