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

const DistributionCard = {
    template: `
        <div class="chart-card">
            <canvas id="distributionChart"></canvas>
        </div>
    `
}

const app = createApp({
    setup() {
        const proxies = ref([])
        const dailyStats = ref([])
        const serviceDistribution = ref([])
        const sortOrder = ref('desc')
        const isDark = ref(false)
        const requestsChartInstance = ref(null)
        const distributionChartInstance = ref(null)
        const loading = ref(true)

        // 计算所有有效代理的平均响应时间
        const avgResponseTime = computed(() => {
            const validProxies = proxies.value.filter(p => p.request_count > 0 && p.response_time > 0)
            if (validProxies.length === 0) return 0
            const sum = validProxies.reduce((acc, p) => acc + p.response_time, 0)
            return Math.round(sum / validProxies.length)
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

        const getChartColors = () => {
            return {
                primary: '#0ea5e9',
                secondary: '#06b6d4',
                accent: '#1fb6ff',
                grid: isDark.value ? 'rgba(226, 232, 240, 0.08)' : 'rgba(15, 23, 42, 0.08)',
                ticks: isDark.value ? '#e5e7eb' : '#0f172a',
                tooltipBg: isDark.value ? 'rgba(15, 23, 42, 0.9)' : 'rgba(255, 255, 255, 0.95)',
                tooltipText: isDark.value ? '#f1f5f9' : '#0f172a'
            }
        }

        const toggleDarkMode = () => {
            isDark.value = !isDark.value;
            localStorage.setItem('darkMode', String(isDark.value));
            updateHtmlClass(isDark.value);
            if (document.getElementById('requestsChart')) {
                initDailyChart();
            }
            if (document.getElementById('distributionChart')) {
                initDistributionChart();
            }
        };

        const initDailyChart = () => {
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

            const labels = dailyStats.value.map(item => item.date)
            const data = dailyStats.value.map(item => item.request_count)
            const colors = getChartColors()

            if (!labels.length) {
                return
            }
            try {
                requestsChartInstance.value = new Chart(ctx, {
                    type: 'bar',
                    data: {
                        labels,
                        datasets: [{
                            label: '调用次数',
                            data,
                            backgroundColor: colors.primary,
                            borderRadius: 10,
                            borderSkipped: false,
                            barThickness: 26,
                            maxBarThickness: 32,
                        }]
                    },
                    options: {
                        responsive: true,
                        layout: {
                            padding: {
                                top: 12,
                                bottom: 8,
                                left: 4,
                                right: 4,
                            }
                        },
                        scales: {
                            x: {
                                grid: {
                                    color: colors.grid,
                                    drawTicks: false,
                                    drawBorder: false,
                                },
                                ticks: {
                                    color: colors.ticks,
                                    maxRotation: 45,
                                    minRotation: 30,
                                    font: {
                                        family: 'Inter, system-ui, sans-serif',
                                        size: 12,
                                    }
                                }
                            },
                            y: {
                                beginAtZero: true,
                                grid: {
                                    color: colors.grid,
                                    drawBorder: false,
                                },
                                ticks: {
                                    color: colors.ticks,
                                    font: {
                                        family: 'Inter, system-ui, sans-serif',
                                        size: 12,
                                    }
                                }
                            }
                        },
                        plugins: {
                            legend: { display: false },
                            title: {
                                display: true,
                                text: '最近一周每日调用次数',
                                color: colors.ticks,
                                font: {
                                    weight: '600',
                                    size: 16,
                                }
                            },
                            tooltip: {
                                backgroundColor: colors.tooltipBg,
                                titleColor: colors.tooltipText,
                                bodyColor: colors.tooltipText,
                                borderColor: isDark.value ? 'rgba(59, 130, 246, 0.35)' : 'rgba(14, 165, 233, 0.25)',
                                borderWidth: 1,
                                callbacks: {
                                    label: function(context) {
                                        const label = context.label || ''
                                        const value = context.raw || 0
                                        return `${label}: ${value} 次请求`
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

        const initDistributionChart = () => {
            const ctx = document.getElementById('distributionChart')
            if (!ctx) {
                console.error('找不到 distributionChart 元素')
                return
            }
            if (typeof Chart === 'undefined') {
                console.error('Chart.js 未加载')
                return
            }
            if (distributionChartInstance.value) {
                distributionChartInstance.value.destroy();
            }

            if (!serviceDistribution.value.length) {
                return
            }

            const labels = serviceDistribution.value.map(item => item.service_name)
            const data = serviceDistribution.value.map(item => item.request_count)
            const palette = ['#0ea5e9', '#06b6d4', '#22c55e', '#a855f7', '#f59e0b', '#ef4444', '#14b8a6']
            const colors = getChartColors()

            try {
                distributionChartInstance.value = new Chart(ctx, {
                    type: 'doughnut',
                    data: {
                        labels,
                        datasets: [{
                            data,
                            backgroundColor: labels.map((_, idx) => palette[idx % palette.length]),
                            borderColor: isDark.value ? 'rgba(15, 23, 42, 0.9)' : 'rgba(255, 255, 255, 0.9)',
                            borderWidth: 2,
                            hoverOffset: 6,
                            cutout: '64%',
                        }]
                    },
                    options: {
                        responsive: true,
                        plugins: {
                            legend: {
                                display: true,
                                labels: {
                                    color: colors.ticks,
                                    usePointStyle: true,
                                    pointStyle: 'roundedRect',
                                    boxWidth: 14,
                                    boxHeight: 10,
                                    padding: 16,
                                    font: {
                                        family: 'Inter, system-ui, sans-serif',
                                        size: 12,
                                    }
                                }
                            },
                            title: {
                                display: true,
                                text: '最近一周请求分布',
                                color: colors.ticks,
                                font: {
                                    weight: '600',
                                    size: 16,
                                }
                            },
                            tooltip: {
                                backgroundColor: colors.tooltipBg,
                                titleColor: colors.tooltipText,
                                bodyColor: colors.tooltipText,
                                borderColor: isDark.value ? 'rgba(59, 130, 246, 0.35)' : 'rgba(14, 165, 233, 0.25)',
                                borderWidth: 1,
                                callbacks: {
                                    label: function(context) {
                                        const label = context.label || ''
                                        const value = context.raw || 0
                                        return `${label}: ${value} 次请求`
                                    }
                                }
                            }
                        }
                    }
                })
            } catch (error) {
                console.error('初始化请求分布图表时出错:', error);
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
                const [proxyRes, dailyRes, distRes] = await Promise.all([
                    fetch('/api/stats'),
                    fetch('/api/stats/daily'),
                    fetch('/api/stats/distribution')
                ])

                proxies.value = proxyRes.ok ? await proxyRes.json() : []
                dailyStats.value = dailyRes.ok ? await dailyRes.json() : []
                serviceDistribution.value = distRes.ok ? await distRes.json() : []
            } catch (error) {
                console.error('获取代理统计信息时出错:', error);
            } finally {
                loading.value = false;
                // 等待 UI 从 loading 切换到内容再初始化图表
                await Vue.nextTick();
                initDailyChart();
                initDistributionChart();
            }
        });

        return {
            proxies,
            sortOrder,
            isDark,
            loading,
            avgResponseTime,
            sortedProxies,
            dailyStats,
            serviceDistribution,
            sortByRequests,
            toggleDarkMode,
        }
    }
})

// 注册组件
app.component('proxy-list-item', ProxyListItem)
app.component('stat-card', StatCard)
app.component('chart-card', ChartCard)
app.component('distribution-card', DistributionCard)

app.mount('#app')
