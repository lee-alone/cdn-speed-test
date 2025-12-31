// App State
let isTesting = false;
let results = [];
let currentConfig = {};
let showQualifiedOnly = false;
let speedChart = null;
let metricsInterval = null;
let datacenters = [];
let testStartTime = null;
let testTimeoutId = null;
let lastProgressUpdate = null;
let finalResultsFetched = false;

// Helpers
function parseAndSortDatacenters(rawDatacenters) {
    const parsed = rawDatacenters.map(dc => {
        let code = '', location = '';
        if (typeof dc === 'object' && dc !== null) {
            code = dc.code || '';
            location = dc.location || '';
        } else if (typeof dc === 'string') {
            const codeMatch = dc.match(/\(([A-Z0-9]+)\)$/);
            if (codeMatch) {
                code = codeMatch[1];
                location = dc.replace(/\s*\([A-Z0-9]+\)\s*$/, '').trim();
            } else {
                code = dc; location = dc;
            }
        }
        return { code, location, displayName: `${code} - ${location}` };
    });
    return parsed.sort((a, b) => a.code.localeCompare(b.code));
}

// Global Exports for HTML onclick
window.startTestWithAutoSave = async function () {
    if (isTesting) return;
    UI.setButtonDisabled('startBtn', true);

    try {
        const urls = {};
        document.querySelectorAll('[data-url-key]').forEach(input => {
            const key = input.getAttribute('data-url-key'), value = input.value.trim();
            if (value) urls[key] = value;
        });

        const selectedDatacenters = Array.from(document.querySelectorAll('input[name="datacenter"]:checked')).map(cb => cb.value);

        const config = {
            test: {
                expected_servers: parseInt(document.getElementById('expectedServers').value) || 3,
                use_tls: document.getElementById('useTLS').checked,
                ip_type: document.getElementById('ipType').value || 'ipv4',
                bandwidth: parseFloat(document.getElementById('bandwidth').value) || 100,
                timeout: parseInt(document.getElementById('timeout').value) || 5,
                download_time: parseInt(document.getElementById('downloadTime').value) || 10,
                file_path: './',
                datacenter_filter: document.getElementById('datacenterMode').value || 'all',
                concurrent_workers: parseInt(document.getElementById('concurrentWorkers').value) || 10,
                sample_interval: 1
            },
            download: { urls },
            ui: {
                datacenter_filter: document.getElementById('datacenterMode').value || 'all',
                result_format: 'table', auto_refresh: true, theme: 'light'
            },
            advanced: {
                concurrent_workers: parseInt(document.getElementById('concurrentWorkers').value) || 10,
                retry_attempts: parseInt(document.getElementById('maxRetries').value) || 3,
                log_level: 'info',
                enable_metrics: document.getElementById('enableMetrics').checked
            }
        };

        const validateResult = await API.validateConfig(config);
        if (!validateResult.valid) throw new Error(validateResult.error || '配置验证失败');

        await API.updateConfig(config);
        await API.updateDatacenterFilter(config.ui.datacenter_filter, selectedDatacenters);
        await API.saveConfig();

        startTest();
    } catch (error) {
        console.error('Error:', error);
        UI.showAlert('配置保存失败: ' + error.message);
        UI.setButtonDisabled('startBtn', false);
    }
};

async function startTest() {
    if (isTesting) return;
    isTesting = true;
    testStartTime = Date.now();
    lastProgressUpdate = Date.now();
    UI.setButtonDisabled('startBtn', true);
    UI.setButtonDisabled('stopBtn', false);
    UI.updateElementText('testStatus', '测试中...');

    testTimeoutId = setTimeout(() => {
        if (isTesting) {
            UI.showAlert('测试超时，自动停止。请检查网络连接或配置。');
            stopTest();
        }
    }, 30 * 60 * 1000);

    if (document.getElementById('enableMetrics').checked) {
        UI.updateElementDisplay('speedChartContainer', 'block');
        UI.updateElementDisplay('metricsContainer', 'block');
        speedChart = createSpeedChart('speedChart');
        startMetricsPolling();
    }

    try {
        await API.startTest();
        pollResults();
        pollStats();
    } catch (error) {
        console.error('Error:', error);
        UI.showAlert('启动测试失败: ' + error.message);
        stopTestUI();
    }
}

window.stopTest = async function () {
    try {
        await API.stopTest();
        stopTestUI();
    } catch (error) {
        console.error('Error:', error);
    }
};

function stopTestUI() {
    isTesting = false; testStartTime = null; lastProgressUpdate = null; finalResultsFetched = false;
    if (testTimeoutId) { clearTimeout(testTimeoutId); testTimeoutId = null; }
    UI.setButtonDisabled('startBtn', false);
    UI.setButtonDisabled('stopBtn', true);
    UI.updateElementText('testStatus', '已停止');
    if (metricsInterval) { clearInterval(metricsInterval); metricsInterval = null; }
}

window.clearResults = async function () {
    try {
        await API.clearResults();
        results = [];
        UI.renderResults('resultsContainer', results);
        resetStats();
        if (speedChart) { speedChart.data = []; speedChart.draw(); }
    } catch (error) { console.error('Error:', error); }
};

function resetStats() {
    UI.updateElementText('progress', '0/0');
    UI.updateElementText('currentSpeed', '- Mbps');
    UI.updateElementText('avgSpeed', '- Mbps');
    UI.updateElementText('peakSpeed', '- Mbps');
}

window.updateData = async function () {
    try {
        const statusData = await API.getStatus();
        const hasAllFiles = statusData.missing_files && statusData.missing_files.length === 0;

        if (hasAllFiles) {
            if (!UI.confirm('本地已存在数据文件，是否要覆盖更新？\n\n点击"确定"将下载最新数据并覆盖原文件。')) return;
        }

        UI.setButtonDisabled('updateBtn', true);
        UI.updateElementText('updateBtn', '更新中...');

        const data = await API.updateData(hasAllFiles);
        if (data.files > 0) UI.showAlert('数据更新已开始，请稍候...');
        else UI.showAlert('所有数据文件已是最新');

        setTimeout(checkStatus, 2000);
    } catch (error) {
        UI.showAlert('更新失败: ' + error.message);
    } finally {
        UI.setButtonDisabled('updateBtn', false);
        UI.updateElementText('updateBtn', '更新数据');
    }
};

window.showExportOptions = () => UI.updateElementDisplay('exportModal', 'block');
window.hideExportOptions = () => UI.updateElementDisplay('exportModal', 'none');

window.exportResults = async function () {
    const format = document.getElementById('exportFormat').value;
    const sort = document.getElementById('exportSort').value;
    const ascending = document.getElementById('exportAscending').checked;
    const order = ascending ? 'asc' : 'desc';
    const url = `/api/results/export/${format}?sort=${sort}&order=${order}`;

    try {
        const response = await fetch(url);
        if (!response.ok) throw new Error('Export failed');
        const blob = await response.blob();
        const downloadUrl = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = downloadUrl;
        a.download = `cloudflare-speedtest-${new Date().toISOString().slice(0, 19).replace(/:/g, '-')}.${format}`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        window.URL.revokeObjectURL(downloadUrl);
        window.hideExportOptions();
    } catch (error) {
        UI.showAlert('导出失败: ' + error.message);
    }
};

window.toggleDatacenterSelection = async function () {
    const mode = document.getElementById('datacenterMode').value;
    UI.updateElementDisplay('datacenterSelection', mode === 'selected' ? 'block' : 'none');
    if (mode === 'selected' && datacenters.length === 0) loadDatacenters();
};

window.updateSelectedCount = () => {
    const selected = document.querySelectorAll('input[name="datacenter"]:checked').length;
    UI.updateElementText('selectedCount', selected);
};

window.toggleAdvancedConfig = () => {
    const container = document.getElementById('advancedConfig');
    const button = document.getElementById('toggleAdvanced');
    if (container.style.display === 'none') {
        container.style.display = 'block'; button.textContent = '隐藏高级配置 ▲';
    } else {
        container.style.display = 'none'; button.textContent = '显示高级配置 ▼';
    }
};

window.refreshResults = () => {
    const sort = document.getElementById('resultSort').value;
    const order = document.getElementById('resultOrder').value;
    if (showQualifiedOnly) loadQualifiedResults();
    else loadSortedResults(sort, order);
};

window.toggleQualifiedOnly = () => {
    showQualifiedOnly = !showQualifiedOnly;
    const button = document.getElementById('qualifiedToggle');
    if (showQualifiedOnly) {
        button.textContent = '显示全部'; button.style.background = '#667eea'; button.style.color = 'white';
        loadQualifiedResults();
    } else {
        button.textContent = '仅显示合格'; button.style.background = '#e2e8f0'; button.style.color = '#333';
        window.refreshResults();
    }
};

async function loadSortedResults(sort, order) {
    try {
        const data = await API.getSortedResults(sort, order);
        results = data.results || [];
        UI.renderResults('resultsContainer', results);
    } catch (error) { console.error(error); }
}

async function loadQualifiedResults() {
    try {
        const data = await API.getQualifiedResults();
        results = data.results || [];
        UI.renderResults('resultsContainer', results);
    } catch (error) { console.error(error); }
}

function startMetricsPolling() {
    if (metricsInterval) return;
    metricsInterval = setInterval(async () => {
        const smoothedData = await API.getSmoothedSpeed();
        if (smoothedData) {
            UI.updateElementText('smoothedSpeed', smoothedData.smoothed_speed.toFixed(2) + ' Mbps');
            if (speedChart && smoothedData.smoothed_speed > 0) speedChart.addPoint(smoothedData.smoothed_speed, new Date());
        }
        const samplesData = await API.getSampleCount();
        if (samplesData) UI.updateElementText('sampleCount', samplesData.count);
        updatePerformanceStats();
    }, 2000);
}

async function updatePerformanceStats() {
    try {
        const stats = await API.getStats();
        const progressText = document.getElementById('progress').textContent;
        const qualifiedCount = parseInt(progressText.split('/')[0]) || 0;

        const errorData = await API.getErrorStats();
        let totalErrors = 0;
        if (errorData) {
            const stats = errorData.error_stats || {};
            totalErrors = (stats.network?.total_count || 0) + (stats.timeout?.total_count || 0);
        }

        UI.updateElementText('testsSuccessful', qualifiedCount);
        UI.updateElementText('testsFailed', totalErrors);
        UI.updateElementText('testsTotal', stats.Total || 0);

        const metrics = await API.getPerformanceMetrics();
        if (metrics) {
            UI.updateElementText('avgLatency', metrics.average_latency ? metrics.average_latency.toFixed(2) + ' ms' : '- ms');
            UI.updateElementText('minLatency', metrics.min_latency && metrics.min_latency < 999999 ? metrics.min_latency.toFixed(2) + ' ms' : '- ms');
            UI.updateElementText('maxLatency', metrics.max_latency ? metrics.max_latency.toFixed(2) + ' ms' : '- ms');
            UI.updateElementText('peakSpeed', metrics.peak_speed ? metrics.peak_speed.toFixed(2) + ' Mbps' : '- Mbps');
            UI.updateElementText('dataTransfer', (metrics.total_data_transfer / 1024 / 1024).toFixed(2) + ' MB');
        }
    } catch (error) { console.error(error); }
}

async function checkStatus() {
    try {
        const status = await API.getStatus();
        UI.renderFileStatus('filesStatusContainer', status);
    } catch (error) { console.error(error); }
}

async function loadDatacenters() {
    try {
        const data = await API.getDatacenters();
        datacenters = parseAndSortDatacenters(data.datacenters || []);
        UI.renderDatacenterList('datacenterList', datacenters, window.updateSelectedCount);
        document.getElementById('datacenterMode').value = data.filter_mode || 'all';
        window.toggleDatacenterSelection();
        if (data.selected) {
            data.selected.forEach(code => {
                const cb = document.querySelector(`input[name="datacenter"][value="${code}"]`);
                if (cb) cb.checked = true;
            });
            window.updateSelectedCount();
        }
    } catch (error) { console.error(error); }
}

async function pollResults() {
    try {
        const data = await API.getResults();
        results = data;
        UI.renderResults('resultsContainer', results);
        updateStats();
        if (isTesting) setTimeout(pollResults, 1000);
        else if (!finalResultsFetched) {
            finalResultsFetched = true;
            setTimeout(async () => {
                const final = await API.getResults();
                UI.renderResults('resultsContainer', final);
                updateStats();
            }, 500);
        }
    } catch (error) { if (isTesting) setTimeout(pollResults, 2000); }
}

async function pollStats() {
    if (!isTesting) return;
    try {
        const stats = await API.getStats();
        const expectedServers = currentConfig.test?.expected_servers || 3;
        if (stats.Total > 0) {
            UI.updateElementText('progress', `${stats.Qualified}/${expectedServers}`);
            if (stats.Qualified > 0) lastProgressUpdate = Date.now();
            else if (lastProgressUpdate && (Date.now() - lastProgressUpdate) > 5 * 60 * 1000) {
                UI.updateElementText('testStatus', '测试可能已停滞，请考虑重新开始');
            }
        }
        if (stats.Qualified >= expectedServers && stats.Total > 0) {
            UI.updateElementText('testStatus', '已完成'); stopTestUI();
        } else if (stats.CurrentIP) {
            UI.updateElementText('testStatus', `测试中: ${stats.CurrentIP}`);
        }
        if (isTesting) setTimeout(pollStats, 500);
    } catch (error) { if (isTesting) setTimeout(pollStats, 1000); }
}

function updateStats() {
    const expectedServers = currentConfig.test?.expected_servers || 3;
    const completed = results.filter(r => r.Status === '已完成').length;
    UI.updateElementText('progress', `${completed}/${expectedServers}`);
    const speeds = results.filter(r => r.Status === '已完成' && r.Speed !== '-').map(r => parseFloat(r.Speed));
    if (speeds.length > 0) {
        UI.updateElementText('avgSpeed', (speeds.reduce((a, b) => a + b, 0) / speeds.length).toFixed(2) + ' Mbps');
        UI.updateElementText('currentSpeed', speeds[speeds.length - 1].toFixed(2) + ' Mbps');
    }
}

// Initial Load
window.addEventListener('load', async () => {
    try {
        currentConfig = await API.getConfig();
        document.getElementById('expectedServers').value = currentConfig.test?.expected_servers || 3;
        document.getElementById('useTLS').checked = currentConfig.test?.use_tls || false;
        document.getElementById('ipType').value = currentConfig.test?.ip_type || 'ipv4';
        document.getElementById('bandwidth').value = currentConfig.test?.bandwidth || 100;
        document.getElementById('timeout').value = currentConfig.test?.timeout || 5;
        document.getElementById('downloadTime').value = currentConfig.test?.download_time || 10;
        document.getElementById('concurrentWorkers').value = currentConfig.advanced?.concurrent_workers || 10;
        document.getElementById('maxRetries').value = currentConfig.advanced?.retry_attempts ?? -1;
        document.getElementById('enableMetrics').checked = currentConfig.advanced?.enable_metrics || false;
        document.getElementById('datacenterMode').value = currentConfig.ui?.datacenter_filter || 'all';

        UI.renderURLs('urlsContainer', currentConfig.download?.urls || {});
        await loadDatacenters();
        UI.renderResults('resultsContainer', []);
        checkStatus();

        // Setup event listeners for sorting elements after they are confirmed to exist
        document.getElementById('datacenterMode').addEventListener('change', window.toggleDatacenterSelection);
        document.getElementById('toggleAdvanced').addEventListener('click', window.toggleAdvancedConfig);
        document.getElementById('resultSort').addEventListener('change', window.refreshResults);
        document.getElementById('resultOrder').addEventListener('change', window.refreshResults);
    } catch (error) { console.error('Init Error:', error); }
});
