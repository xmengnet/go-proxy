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
                    toast.className = 'fixed bottom-4 right-4 bg-green-500 text-white px-6 py-3 rounded-lg shadow-lg transform transition-all duration-300 translate-y-0 opacity-100'
                    toast.textContent = '已复制到剪贴板'
                    document.body.appendChild(toast)
                    setTimeout(() => {
                        toast.classList.add('translate-y-2', 'opacity-0')
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
        <div class="bg-gray-50 dark:bg-gray-700 rounded-lg p-4 hover:shadow-md transition-shadow duration-200">
            <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between">
                <div class="flex items-center space-x-4">
                    <img :src="getVendorIcon(proxy.vendor)" :alt="proxy.vendor" class="w-8 h-8">
                    <div>
                        <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ getFullProxyUrl(proxy.service_name) }}</h3>
                        <p class="text-sm text-gray-500 dark:text-gray-400">{{ proxy.target }}</p>
                    </div>
                </div>
                <div class="mt-2 flex w-full items-center justify-between sm:mt-0 sm:w-auto sm:justify-start sm:space-x-4">
                    <span class="px-3 py-1 bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200 rounded-full text-sm">
                        {{ proxy.request_count }} 请求
                    </span>
                    <button @click="copyProxyUrl(proxy)" class="px-4 py-2 bg-secondary text-white rounded-lg hover:bg-green-600 transition-colors duration-200">
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
        valueColorClass: {
            type: String,
            default: 'text-primary' // 默认颜色
        }
    },
    template: `
        <div class="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-2">{{ title }}</h3>
            <p class="text-3xl font-bold" :class="valueColorClass">{{ value }}</p>
        </div>
    `
}

// 定义 ChartCard 组件
const ChartCard = {
    template: `
        <div class="bg-white dark:bg-gray-800 rounded-lg shadow p-6 flex justify-center items-center">
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
                // 使用 nextTick 确保 DOM 更新完成
                await Vue.nextTick();
                initChart();
            } catch (error) {
                console.error('获取代理统计信息时出错:', error);
            }
        });

        return {
            proxies,
            sortOrder,
            isDark,
            totalRequests,
            activeProxies,
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
