const { createApp, ref, computed, onMounted } = Vue

// 1. 定义 ProxyListItem 组件
const ProxyListItem = {
    props: {
        proxy: {
            type: Object,
            required: true
        }
    },
    setup(props) {
        // 将与单个列表项相关的方法移到这里
        const getFullProxyUrl = (path) => {
            return `${window.location.protocol}//${window.location.host}${path}`
        }

        const copyProxyUrl = (proxyItem) => { // 注意：这里接收的参数是 props.proxy
            const fullUrl = getFullProxyUrl(proxyItem.service_name)
            navigator.clipboard.writeText(fullUrl)
                .then(() => {
                    const toast = document.createElement('div')
                    toast.className = 'toast-notification'
                    toast.textContent = '已复制到剪贴板'
                    document.body.appendChild(toast)
                    setTimeout(() => {
                        toast.classList.add('toast-hide')
                        setTimeout(() => {
                            document.body.removeChild(toast)
                        }, 300)
                    }, 2000)
                })
                .catch(err => console.error('复制失败:', err))
        }

        const getVendorIcon = (vendor) => {
            const defaultVendor = 'openai'
            const vendorName = vendor || defaultVendor
            return `https://unpkg.com/@lobehub/icons-static-svg@latest/icons/${vendorName}.svg`
        }

        return {
            // 返回需要在模板中使用的方法和属性
            getFullProxyUrl,
            copyProxyUrl,
            getVendorIcon
        }
    },
    template: `
        <div class="proxy-item">
            <div class="proxy-item-content">
                <div class="proxy-item-info">
                    <img :src="getVendorIcon(proxy.vendor)" :alt="proxy.vendor" class="vendor-icon-img">
                    <div class="proxy-item-details">
                        <h3 class="proxy-item-title">{{ getFullProxyUrl(proxy.service_name) }}</h3>
                        <p class="proxy-item-target">{{ proxy.target }}</p>
                    </div>
                </div>
                <div class="proxy-item-stats">
                    <span class="stat-badge stat-badge-requests">
                        {{ proxy.request_count }} 请求
                    </span>
                    <span class="stat-badge stat-badge-time">
                        {{ Math.round(proxy.response_time) }}ms
                    </span>
                    <button @click="copyProxyUrl(proxy)" class="copy-proxy-btn">
                        复制地址
                    </button>
                </div>
            </div>
        </div>
    `
}

// 定义 StatCard 组件
const StatCard = {
    props: {
        title: String,
        value: [String, Number],
        unit: {
            type: String,
            default: ''
        },
        tooltip: {
            type: String,
            default: ''
        },
        valueColorClass: {
            type: String,
            default: 'text-primary' // 默认颜色
        }
    },
    template: `
        <div class="stat-card stat-card-dashboard relative group">
            <h3 class="stat-card-title">{{ title }}</h3>
            <p class="stat-card-value" :class="valueColorClass">{{ value }}{{ unit }}</p>
            <div v-if="tooltip" class="stat-card-tooltip">
                {{ tooltip }}
            </div>
        </div>
    `
}

// 定义 ChartCard 组件
const ChartCard = {
    template: `
        <div class="chart-card">
            <canvas id="requestsChart"></canvas>
        </div>
    `
}

const app = createApp({
    setup() {
        const proxies = ref([])
        const sortOrder = ref('desc')
        const isDark = ref(false)
        const requestsChartInstance = ref(null)
        const loading = ref(true)

        // 计算API调用总成本（示例：每次调用0.015元）
        const totalCost = computed(() => {
            const cost = proxies.value.reduce((sum, proxy) => sum + proxy.request_count * 0.015, 0)
            return cost.toFixed(2)
        })

        // 计算所有有效代理的平均响应时间
        const avgResponseTime = computed(() => {
            const validProxies = proxies.value.filter(p => p.request_count > 0 && p.response_time > 0)
            if (validProxies.length === 0) return 0
            const sum = validProxies.reduce((acc, p) => acc + p.response_time, 0)
            return Math.round(sum / validProxies.length)
        })

        const totalRequests = computed(() => {
            return proxies.value.reduce((sum, proxy) => sum + proxy.request_count, 0)
        })

        const activeProxies = computed(() => {
            return proxies.value.filter(proxy => proxy.request_count > 0).length
        })

        const sortedProxies = computed(() => {
            return [...proxies.value].sort((a, b) => {
                return sortOrder.value === 'asc'
                    ? a.request_count - b.request_count
                    : b.request_count - a.request_count
            })
        })

        const sortByRequests = () => {
            sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
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
            if (document.getElementById('requestsChart')) {
                initChart();
            }
        };

        const initChart = () => {
            const ctx = document.getElementById('requestsChart')
            if (!ctx) {
                console.error('找不到 requestsChart 元素')
                return
            }
            if (typeof Chart === 'undefined') {
                console.error('Chart.js 未加载')
                return
            }
            if (requestsChartInstance.value) {
                requestsChartInstance.value.destroy();
            }
            try {
                requestsChartInstance.value = new Chart(ctx, {
                    type: 'doughnut',
                    data: {
                        labels: proxies.value.map(p => p.service_name),
                        datasets: [{
                            data: proxies.value.map(p => p.request_count),
                            backgroundColor: [
                                '#3B82F6', '#10B981', '#F59E0B', '#EF4444', '#8B5CF6',
                            ]
                        }]
                    },
                    options: {
                        responsive: true,
                        plugins: {
                            legend: { display: false },
                            title: {
                                display: true,
                                text: '请求分布',
                                color: isDark.value ? '#FFF' : '#374151'
                            },
                            tooltip: {
                                callbacks: {
                                    label: function(context) {
                                        const label = context.label || '';
                                        const value = context.raw || 0;
                                        return `${label}: ${value} 次请求`;
                                    }
                                }
                            }
                        }
                    }
                })
            } catch (error) {
                console.error('初始化图表时出错:', error);
            }
        }

        onMounted(async () => {
            const storedDarkMode = localStorage.getItem('darkMode');
            if (storedDarkMode !== null) {
                isDark.value = (storedDarkMode === 'true');
            } else {
                isDark.value = document.documentElement.classList.contains('dark');
            }
            updateHtmlClass(isDark.value);

            try {
                const response = await fetch('/api/stats');
                proxies.value = await response.json();
            } catch (error) {
                console.error('获取代理统计信息时出错:', error);
            } finally {
                loading.value = false;
                // 等待 UI 从 loading 切换到内容再初始化图表
                await Vue.nextTick();
                initChart();
            }
        });

        return {
            proxies,
            sortOrder,
            isDark,
            loading,
            totalRequests,
            totalCost,
            avgResponseTime,
            sortedProxies,
            sortByRequests,
            toggleDarkMode,
        }
    }
})

// 注册组件
app.component('proxy-list-item', ProxyListItem)
app.component('stat-card', StatCard)
app.component('chart-card', ChartCard)

app.mount('#app')
