        const baseUrl = 'http://localhost:8097';

        function formatBytes(bytes) {
            if (!bytes && bytes !== 0) return '--';
            if (bytes === 0) return '0 B';
            const k = 1024;
            const sizes = ['B', 'KB', 'MB', 'GB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
        }

        function formatDuration(milliseconds) {
            if (typeof milliseconds !== 'number' || Number.isNaN(milliseconds)) {
                return '-- ms';
            }
            return `${milliseconds.toFixed(2)} ms`;
        }

        function formatTimestamp(timestamp) {
            if (!timestamp) {
                return '--';
            }
            const date = new Date(timestamp);
            return date.toLocaleString();
        }

        function updateMetricsDashboard(data) {
            document.getElementById('uptime').textContent = data.uptime || 'N/A';
            document.getElementById('service-status').textContent = data.database_status || 'Unknown';
            const statusElement = document.getElementById('service-status');
            statusElement.className = 'metric-value status-chip status-' + (data.database_status || 'disconnected').toLowerCase();
            document.getElementById('db-latency').textContent = formatDuration(data.database_latency_ms);
            document.getElementById('cpu-count').textContent = data.cpu_count || '--';
            document.getElementById('goroutines').textContent = data.goroutines || '--';
            document.getElementById('max-procs').textContent = data.max_procs || '--';
            document.getElementById('memory-usage').textContent = data.memory_stats ? formatBytes(data.memory_stats.heap_alloc_bytes) : '--';
            document.getElementById('gc-count').textContent = data.memory_stats ? data.memory_stats.num_gc : '--';
            document.getElementById('go-version').textContent = data.go_version || '--';
            document.getElementById('last-updated').textContent = formatTimestamp(data.timestamp);

            const memoryBreakdown = document.getElementById('memory-breakdown');
            let breakdownHtml = '';
            if (data.memory_stats) {
                breakdownHtml += `
                    <div class="memory-item"><span>Heap Alloc:</span><span>${formatBytes(data.memory_stats.heap_alloc_bytes)}</span></div>
                    <div class="memory-item"><span>Heap Sys:</span><span>${formatBytes(data.memory_stats.heap_sys_bytes)}</span></div>
                    <div class="memory-item"><span>Stack Inuse:</span><span>${formatBytes(data.memory_stats.stack_inuse_bytes)}</span></div>
                    <div class="memory-item"><span>Total Alloc:</span><span>${formatBytes(data.memory_stats.total_alloc_bytes)}</span></div>
                `;
            }
            if (data.system_memory) {
                breakdownHtml += `
                    <div class="memory-item"><span>System Used:</span><span>${formatBytes(data.system_memory.used_bytes)}</span></div>
                    <div class="memory-item"><span>System Free:</span><span>${formatBytes(data.system_memory.available_bytes)}</span></div>
                    <div class="memory-item"><span>Used %:</span><span>${data.system_memory.used_percent.toFixed(1)}%</span></div>
                `;
            }
            memoryBreakdown.innerHTML = breakdownHtml;
        }

        async function fetchEndpoint(endpoint, elementId) {
            const element = document.getElementById(elementId);
            try {
                const startTime = performance.now();
                const response = await fetch(`${baseUrl}${endpoint}`);

                if (endpoint === '/metrics') {
                    const data = await response.json();
                    updateMetricsDashboard(data);
                    element.textContent = JSON.stringify(data, null, 2);
                } else {
                    const text = await response.text();
                    const endTime = performance.now();
                    const latency = (endTime - startTime).toFixed(2);
                    element.textContent = `${text}\n\nLatency: ${latency} ms`;
                    if (endpoint === '/health') {
                        document.getElementById('health-latency').textContent = formatDuration(parseFloat(latency));
                    }
                }
            } catch (error) {
                element.innerHTML = `Error: ${error.message}`;
            }
        }

        async function fetchAllEndpoints() {
            await Promise.all([
                fetchEndpoint('/health', 'health-response'),
                fetchEndpoint('/metrics', 'metrics-response')
            ]);
        }

        // Auto-refresh every 30 seconds
        setInterval(fetchAllEndpoints, 30000);

        // Initial load
        window.onload = function() {
            fetchAllEndpoints();
        };