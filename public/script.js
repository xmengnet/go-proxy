const { createApp, ref, computed, onMounted, watch } = Vue

// 1. 定义 ProxyListItem 组件
const ProxyListItem = {
    props: {
        proxy: {
            type: Object,
            required: true
        }
    },
    setup(props) {
        const getFullProxyUrl = (path) => {
            return `${window.location.protocol}//${window.location.host}${path}`
        }

        const copyProxyUrl = (proxyItem) => {
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
            default: 'stat-value-primary'
        }
    },
    template: `
        <div class="stat-card-dashboard">
            <h3 class="stat-card-title">{{ title }}</h3>
            <p class="stat-card-value" :class="valueColorClass">{{ value }}{{ unit }}</p>
            <div v-if="tooltip" class="stat-card-tooltip">
                {{ tooltip }}
            </div>
        </div>
    `
}

// 定义 ChartCard 组件 - 使用ApexCharts
const ChartCard = {
    template: `
        <div class="chart-card">
            <div id="dailyChart"></div>
        </div>
    `
}

const DistributionCard = {
    template: `
        <div class="chart-card">
            <div id="distributionChart"></div>
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
        const dailyChartInstance = ref(null)
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

        // 获取ApexCharts主题配置
        const getChartTheme = () => {
            return {
                mode: isDark.value ? 'dark' : 'light',
                palette: 'palette1',
                monochrome: {
                    enabled: false
                }
            }
        }

        // 获取图表通用配置
        const getChartColors = () => {
            return {
                background: 'transparent',
                foreColor: isDark.value ? '#737373' : '#525252',
                gridColor: isDark.value ? 'rgba(255, 255, 255, 0.06)' : 'rgba(0, 0, 0, 0.06)',
                primary: '#c46b4e',
                gradient: ['#c46b4e', '#d4845c', '#b35a3d'],
                palette: ['#3B82F6', '#10B981', '#c46b4e', '#F59E0B', '#8B5CF6', '#06B6D4']
            }
        }

        const toggleDarkMode = () => {
            isDark.value = !isDark.value;
            localStorage.setItem('darkMode', String(isDark.value));
            updateHtmlClass(isDark.value);
            // 重新渲染图表以适应主题变化
            initDailyChart();
            initDistributionChart();
        };

        // 初始化每日调用次数图表 - 使用面积图替代柱状图
        const initDailyChart = () => {
            const chartEl = document.getElementById('dailyChart')
            if (!chartEl) {
                console.error('找不到 dailyChart 元素')
                return
            }
            if (typeof ApexCharts === 'undefined') {
                console.error('ApexCharts 未加载')
                return
            }

            // 销毁旧图表
            if (dailyChartInstance.value) {
                dailyChartInstance.value.destroy();
            }

            const labels = dailyStats.value.map(item => {
                // 格式化日期显示为 MM-DD
                const date = new Date(item.date)
                return `${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
            })
            const data = dailyStats.value.map(item => item.request_count)
            const colors = getChartColors()

            if (!labels.length) {
                return
            }

            const options = {
                series: [{
                    name: '调用次数',
                    data: data
                }],
                chart: {
                    type: 'area',
                    height: 280,
                    background: colors.background,
                    foreColor: colors.foreColor,
                    fontFamily: 'Inter, system-ui, sans-serif',
                    toolbar: {
                        show: false
                    },
                    animations: {
                        enabled: true,
                        easing: 'easeinout',
                        speed: 800,
                        animateGradually: {
                            enabled: true,
                            delay: 150
                        },
                        dynamicAnimation: {
                            enabled: true,
                            speed: 350
                        }
                    },
                    dropShadow: {
                        enabled: false
                    }
                },
                colors: [colors.primary],
                fill: {
                    type: 'gradient',
                    gradient: {
                        shadeIntensity: 1,
                        opacityFrom: 0.45,
                        opacityTo: 0.05,
                        stops: [0, 100],
                        colorStops: [
                            {
                                offset: 0,
                                color: colors.primary,
                                opacity: 0.4
                            },
                            {
                                offset: 100,
                                color: colors.primary,
                                opacity: 0.05
                            }
                        ]
                    }
                },
                stroke: {
                    curve: 'smooth',
                    width: 3,
                    lineCap: 'round'
                },
                dataLabels: {
                    enabled: false
                },
                title: {
                    text: '最近一周每日调用次数',
                    align: 'left',
                    style: {
                        fontSize: '16px',
                        fontWeight: 600,
                        color: isDark.value ? '#F1F5F9' : '#1E293B'
                    }
                },
                grid: {
                    show: true,
                    borderColor: colors.gridColor,
                    strokeDashArray: 4,
                    padding: {
                        left: 10,
                        right: 10
                    }
                },
                xaxis: {
                    categories: labels,
                    axisBorder: {
                        show: false
                    },
                    axisTicks: {
                        show: false
                    },
                    labels: {
                        style: {
                            colors: colors.foreColor,
                            fontSize: '12px'
                        }
                    },
                    crosshairs: {
                        show: true,
                        stroke: {
                            color: colors.primary,
                            width: 1,
                            dashArray: 3
                        }
                    }
                },
                yaxis: {
                    labels: {
                        style: {
                            colors: colors.foreColor,
                            fontSize: '12px'
                        },
                        formatter: (value) => {
                            if (value >= 1000) {
                                return (value / 1000).toFixed(1) + 'k'
                            }
                            return Math.round(value)
                        }
                    }
                },
                markers: {
                    size: 0,
                    strokeWidth: 0,
                    hover: {
                        size: 6,
                        sizeOffset: 3
                    }
                },
                tooltip: {
                    theme: isDark.value ? 'dark' : 'light',
                    x: {
                        show: true
                    },
                    y: {
                        formatter: (value) => `${value.toLocaleString()} 次请求`
                    },
                    marker: {
                        show: true
                    }
                }
            }

            try {
                dailyChartInstance.value = new ApexCharts(chartEl, options)
                dailyChartInstance.value.render()
            } catch (error) {
                console.error('初始化每日图表时出错:', error)
            }
        }

        // 初始化请求分布图表 - 使用径向条形图
        const initDistributionChart = () => {
            const chartEl = document.getElementById('distributionChart')
            if (!chartEl) {
                console.error('找不到 distributionChart 元素')
                return
            }
            if (typeof ApexCharts === 'undefined') {
                console.error('ApexCharts 未加载')
                return
            }

            // 销毁旧图表
            if (distributionChartInstance.value) {
                distributionChartInstance.value.destroy();
            }

            if (!serviceDistribution.value.length) {
                return
            }

            const colors = getChartColors()

            // 计算总请求数用于百分比
            const totalRequests = serviceDistribution.value.reduce((sum, item) => sum + item.request_count, 0)

            // 准备数据
            const labels = serviceDistribution.value.map(item => item.service_name)
            const data = serviceDistribution.value.map(item => item.request_count)
            const percentages = serviceDistribution.value.map(item =>
                Math.round((item.request_count / totalRequests) * 100)
            )

            const options = {
                series: percentages,
                chart: {
                    type: 'donut',
                    height: 280,
                    background: colors.background,
                    foreColor: colors.foreColor,
                    fontFamily: 'Inter, system-ui, sans-serif',
                    animations: {
                        enabled: true,
                        easing: 'easeinout',
                        speed: 800,
                        animateGradually: {
                            enabled: true,
                            delay: 150
                        }
                    },
                    dropShadow: {
                        enabled: true,
                        top: 4,
                        left: 0,
                        blur: 12,
                        opacity: 0.15
                    }
                },
                colors: colors.palette,
                labels: labels,
                title: {
                    text: '请求分布',
                    align: 'left',
                    style: {
                        fontSize: '16px',
                        fontWeight: 600,
                        color: isDark.value ? '#F1F5F9' : '#1E293B'
                    }
                },
                plotOptions: {
                    pie: {
                        donut: {
                            size: '70%',
                            labels: {
                                show: true,
                                name: {
                                    show: true,
                                    fontSize: '14px',
                                    fontWeight: 600,
                                    color: isDark.value ? '#F1F5F9' : '#1E293B',
                                    offsetY: -10
                                },
                                value: {
                                    show: true,
                                    fontSize: '24px',
                                    fontWeight: 700,
                                    color: isDark.value ? '#F1F5F9' : '#1E293B',
                                    offsetY: 6,
                                    formatter: function (val) {
                                        return val + '%'
                                    }
                                },
                                total: {
                                    show: true,
                                    showAlways: true,
                                    label: '总请求',
                                    fontSize: '14px',
                                    fontWeight: 500,
                                    color: colors.foreColor,
                                    formatter: function () {
                                        return totalRequests.toLocaleString()
                                    }
                                }
                            }
                        }
                    }
                },
                stroke: {
                    width: 2,
                    colors: [isDark.value ? '#0a0a0a' : '#FAFAFA']
                },
                dataLabels: {
                    enabled: false
                },
                legend: {
                    show: true,
                    position: 'bottom',
                    horizontalAlign: 'center',
                    offsetY: 8,
                    fontSize: '12px',
                    fontWeight: 500,
                    markers: {
                        width: 10,
                        height: 10,
                        radius: 3,
                        offsetX: -4
                    },
                    itemMargin: {
                        horizontal: 12,
                        vertical: 4
                    },
                    labels: {
                        colors: colors.foreColor
                    },
                    formatter: function (seriesName, opts) {
                        const count = data[opts.seriesIndex]
                        return `${seriesName}: ${count.toLocaleString()}`
                    }
                },
                tooltip: {
                    enabled: true,
                    theme: isDark.value ? 'dark' : 'light',
                    y: {
                        formatter: function (value, { seriesIndex }) {
                            return `${data[seriesIndex].toLocaleString()} 次请求 (${value}%)`
                        }
                    }
                },
                responsive: [{
                    breakpoint: 480,
                    options: {
                        chart: {
                            height: 260
                        },
                        legend: {
                            position: 'bottom',
                            fontSize: '11px'
                        }
                    }
                }]
            }

            try {
                distributionChartInstance.value = new ApexCharts(chartEl, options)
                distributionChartInstance.value.render()
            } catch (error) {
                console.error('初始化请求分布图表时出错:', error)
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
