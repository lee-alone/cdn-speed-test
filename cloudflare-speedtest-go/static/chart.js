function createSpeedChart(canvasId) {
    const canvas = document.getElementById(canvasId);
    if (!canvas) return null;
    const ctx = canvas.getContext('2d');

    return {
        canvas: canvas,
        ctx: ctx,
        data: [],
        maxPoints: 50,

        addPoint: function (speed, timestamp) {
            this.data.push({ speed: speed, timestamp: timestamp });
            if (this.data.length > this.maxPoints) {
                this.data.shift();
            }
            this.draw();
        },

        draw: function () {
            const ctx = this.ctx;
            const canvas = this.canvas;
            const data = this.data;

            ctx.clearRect(0, 0, canvas.width, canvas.height);
            if (data.length < 2) return;

            const maxSpeed = Math.max(...data.map(d => d.speed), 10);
            const padding = 40;
            const chartWidth = canvas.width - padding * 2;
            const chartHeight = canvas.height - padding * 2;

            // Draw grid
            ctx.strokeStyle = '#e2e8f0';
            ctx.lineWidth = 1;
            for (let i = 0; i <= 5; i++) {
                const y = padding + (chartHeight / 5) * i;
                ctx.beginPath();
                ctx.moveTo(padding, y);
                ctx.lineTo(canvas.width - padding, y);
                ctx.stroke();

                ctx.fillStyle = '#666';
                ctx.font = '12px Arial';
                ctx.textAlign = 'right';
                const value = (maxSpeed * (5 - i) / 5).toFixed(1);
                ctx.fillText(value + ' Mbps', padding - 5, y + 4);
            }

            // Draw speed line
            ctx.strokeStyle = '#667eea';
            ctx.lineWidth = 2;
            ctx.beginPath();
            data.forEach((point, index) => {
                const x = padding + (chartWidth / (data.length - 1)) * index;
                const y = padding + chartHeight - (point.speed / maxSpeed) * chartHeight;
                if (index === 0) ctx.moveTo(x, y);
                else ctx.lineTo(x, y);
            });
            ctx.stroke();

            // Draw points
            ctx.fillStyle = '#667eea';
            data.forEach((point, index) => {
                const x = padding + (chartWidth / (data.length - 1)) * index;
                const y = padding + chartHeight - (point.speed / maxSpeed) * chartHeight;
                ctx.beginPath();
                ctx.arc(x, y, 3, 0, 2 * Math.PI);
                ctx.fill();
            });
        }
    };
}
