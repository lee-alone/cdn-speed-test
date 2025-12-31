const UI = {
    updateElementText(id, text) {
        const el = document.getElementById(id);
        if (el) el.textContent = text;
    },

    updateElementDisplay(id, display) {
        const el = document.getElementById(id);
        if (el) el.style.display = display;
    },

    setButtonDisabled(id, disabled) {
        const btn = document.getElementById(id);
        if (btn) btn.disabled = disabled;
    },

    renderResults(containerId, results) {
        const container = document.getElementById(containerId);
        if (!container) return;

        if (!results || results.length === 0) {
            container.innerHTML = '<div class="empty-state">暂无测试结果</div>';
            return;
        }

        let html = `
            <table>
                <thead>
                    <tr>
                        <th>IP地址</th>
                        <th>状态</th>
                        <th>延迟(ms)</th>
                        <th>速度(Mbps)</th>
                        <th>数据中心</th>
                        <th>峰值速度(Mbps)</th>
                    </tr>
                </thead>
                <tbody>
        `;

        results.forEach(result => {
            const statusClass = this.getStatusClass(result.Status);
            html += `
                <tr>
                    <td>${result.IP}</td>
                    <td><span class="status-badge ${statusClass}">${result.Status}</span></td>
                    <td>${result.Latency}</td>
                    <td>${result.Speed}</td>
                    <td>${result.DataCenter}</td>
                    <td>${result.PeakSpeed.toFixed(2)}</td>
                </tr>
            `;
        });

        html += `</tbody></table>`;
        container.innerHTML = html;
    },

    getStatusClass(status) {
        switch (status) {
            case '已完成': return 'status-completed';
            case '测试中': return 'status-testing';
            case '待测试': return 'status-pending';
            case '低速': return 'status-low-speed';
            default: return 'status-error';
        }
    },

    renderDatacenterList(containerId, datacenters, onSelectChange) {
        const container = document.getElementById(containerId);
        if (!container) return;

        if (!datacenters || datacenters.length === 0) {
            container.innerHTML = '<div style="text-align: center; color: #999; padding: 20px;">暂无数据中心信息</div>';
            return;
        }

        let html = '<div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(250px, 1fr)); gap: 10px;">';
        datacenters.forEach(dc => {
            html += `
                <label style="display: flex; align-items: center; padding: 10px; border: 1px solid #e2e8f0; border-radius: 4px; cursor: pointer; background: white; transition: all 0.2s ease;">
                    <input type="checkbox" name="datacenter" value="${dc.code}" style="margin-right: 10px; width: 18px; height: 18px; cursor: pointer;">
                    <div style="flex: 1;">
                        <div style="font-weight: 700; font-size: 13px; color: #667eea;">${dc.code}</div>
                        <div style="font-size: 12px; color: #666; margin-top: 2px;">${dc.location}</div>
                    </div>
                </label>
            `;
        });
        html += '</div>';
        container.innerHTML = html;

        // Add event listeners to newly created checkboxes
        const checkboxes = container.querySelectorAll('input[name="datacenter"]');
        checkboxes.forEach(cb => {
            cb.addEventListener('change', onSelectChange);
        });
    },

    renderURLs(containerId, urls) {
        const container = document.getElementById(containerId);
        if (!container) return;

        if (!urls || Object.keys(urls).length === 0) {
            container.innerHTML = '<div class="empty-state">暂无下载地址配置</div>';
            return;
        }

        let html = '<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 15px;">';
        for (const [key, value] of Object.entries(urls)) {
            html += `
                <div class="config-item">
                    <label for="url_${key}">${key}</label>
                    <input type="text" id="url_${key}" data-url-key="${key}" value="${value}" placeholder="输入下载地址">
                </div>
            `;
        }
        html += '</div>';
        container.innerHTML = html;
    },

    renderFileStatus(containerId, status) {
        const container = document.getElementById(containerId);
        if (!container) return;

        if (!status.missing_files || status.missing_files.length === 0) {
            container.innerHTML = '<div style="color: #22543d; background: #c6f6d5; padding: 10px; border-radius: 4px;">✓ 所有数据文件已就绪</div>';
            return;
        }

        let html = '<div style="color: #7c2d12; background: #feebc8; padding: 10px; border-radius: 4px;">';
        html += '⚠ 缺少以下数据文件: ' + status.missing_files.join(', ');
        html += '<br><small>点击"更新数据"按钮下载缺失的文件</small></div>';
        container.innerHTML = html;
    },

    showAlert(message) {
        alert(message);
    },

    confirm(message) {
        return confirm(message);
    }
};
