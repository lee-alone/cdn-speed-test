const API = {
    async getConfig() {
        const response = await fetch('/api/config');
        if (!response.ok) throw new Error('Failed to load config');
        return response.json();
    },

    async validateConfig(config) {
        const response = await fetch('/api/config/validate', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(config)
        });
        return response.json();
    },

    async updateConfig(config) {
        const response = await fetch('/api/config', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(config)
        });
        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(errorData.error || 'Failed to update config');
        }
        return response.json();
    },

    async updateDatacenterFilter(mode, selected) {
        const response = await fetch('/api/datacenters/filter', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ mode, selected })
        });
        return response.ok;
    },

    async saveConfig() {
        const response = await fetch('/api/config/save', { method: 'POST' });
        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(errorData.error || 'Failed to save config to file');
        }
    },

    async startTest() {
        const response = await fetch('/api/start', { method: 'POST' });
        if (!response.ok) throw new Error('Failed to start test');
    },

    async stopTest() {
        const response = await fetch('/api/stop', { method: 'POST' });
        if (!response.ok) throw new Error('Failed to stop test');
    },

    async clearResults() {
        const response = await fetch('/api/results', { method: 'DELETE' });
        if (!response.ok) throw new Error('Failed to clear results');
    },

    async updateData(force) {
        const response = await fetch('/api/update', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ force })
        });
        if (!response.ok) throw new Error('Failed to update data');
        return response.json();
    },

    async getStatus() {
        const response = await fetch('/api/status');
        if (!response.ok) throw new Error('Failed to get status');
        return response.json();
    },

    async getDatacenters() {
        const response = await fetch('/api/datacenters');
        if (!response.ok) throw new Error('Failed to load datacenters');
        return response.json();
    },

    async getResults() {
        const response = await fetch('/api/results');
        if (!response.ok) throw new Error('Failed to fetch results');
        return response.json();
    },

    async getSortedResults(sort, order) {
        const response = await fetch(`/api/results/sorted?sort=${sort}&order=${order}`);
        if (!response.ok) throw new Error('Failed to load sorted results');
        return response.json();
    },

    async getQualifiedResults() {
        const response = await fetch('/api/results/qualified');
        if (!response.ok) throw new Error('Failed to load qualified results');
        return response.json();
    },

    async getStats() {
        const response = await fetch('/api/stats');
        if (!response.ok) throw new Error('Failed to fetch stats');
        return response.json();
    },

    async getMetrics() {
        const response = await fetch('/api/metrics');
        return response.ok ? response.json() : null;
    },

    async getSmoothedSpeed() {
        const response = await fetch('/api/metrics/speed/smoothed');
        return response.ok ? response.json() : null;
    },

    async getSampleCount(count = 10) {
        const response = await fetch(`/api/metrics/speed/samples?count=${count}`);
        return response.ok ? response.json() : null;
    },

    async getErrorStats() {
        const response = await fetch('/api/errors/stats');
        return response.ok ? response.json() : null;
    },

    async getPerformanceMetrics() {
        const response = await fetch('/api/metrics/performance');
        return response.ok ? response.json() : null;
    }
};
