const { createApp, ref, computed, onMounted } = Vue

const app = createApp({
    setup() {
        const proxies = ref([])
        const sortOrder = ref('desc')
        const isDark = ref(localStorage.getItem('darkMode') === 'true')

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

        const toggleDarkMode = () => {
            isDark.value = !isDark.value
            localStorage.setItem('darkMode', isDark.value)
            document.documentElement.classList.toggle('dark')
        }

        // 添加获取完整 URL 的函数
        const getFullProxyUrl = (path) => {
            return `${window.location.protocol}//${window.location.host}${path}`
        }

        const copyProxyUrl = (proxy) => {
            const fullUrl = getFullProxyUrl(proxy.service_name)
            navigator.clipboard.writeText(fullUrl)
                .then(() => {
                    // 创建并显示提示元素
                    const toast = document.createElement('div')
                    toast.className = 'fixed bottom-4 right-4 bg-green-500 text-white px-6 py-3 rounded-lg shadow-lg transform transition-all duration-300 translate-y-0 opacity-100'
                    toast.textContent = '已复制到剪贴板'
                    document.body.appendChild(toast)

                    // 2秒后移除提示
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
            // 如果没有提供 vendor 或 vendor 为空，默认使用 openai
            const defaultVendor = 'openai'
            const vendorName = vendor || defaultVendor
            return `https://unpkg.com/@lobehub/icons-static-svg@latest/icons/${vendorName}.svg`
        }

        const initChart = () => {
            // 添加延迟以确保 DOM 已加载
            setTimeout(() => {
                const ctx = document.getElementById('requestsChart')
                if (!ctx) {
                    console.error('找不到 requestsChart 元素')
                    return
                }

                new Chart(ctx, {
                    type: 'doughnut',
                    data: {
                        labels: proxies.value.map(p => p.service_name),
                        datasets: [{
                            data: proxies.value.map(p => p.request_count),
                            backgroundColor: [
                                '#3B82F6',
                                '#10B981',
                                '#F59E0B',
                                '#EF4444',
                                '#8B5CF6',
                            ]
                        }]
                    },
                    options: {
                        responsive: true,
                        plugins: {
                            legend: {
                                display: false
                            },
                            title: {
                                display: true,
                                text: '请求分布',
                                color: isDark.value ? '#fff' : '#000'
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
            }, 100) // 添加 100ms 延迟
        }

        onMounted(async () => {
            try {
                const response = await fetch('/api/stats')
                proxies.value = await response.json()
                initChart()
            } catch (error) {
                console.error('获取代理统计信息时出错:', error)
            }
        })

        return {
            proxies,
            sortOrder,
            isDark,
            totalRequests,
            activeProxies,
            sortedProxies,
            sortByRequests,
            toggleDarkMode,
            copyProxyUrl,
            getVendorIcon,
            getFullProxyUrl  // 添加到返回值中
        }
    }
})

app.mount('#app')
