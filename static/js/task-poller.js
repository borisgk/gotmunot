// Generic task poller to be used across different pages.
// It requires a modal with the following element IDs to be present in the HTML:
// - progress-modal
// - progress-title
// - progress-text
// - progress-bar
// - progress-cancel-btn

window.pollTaskStatus = function(taskId, { onComplete, onCancel } = {}) {
    const progressModal = document.getElementById('progress-modal');
    const progressTitle = document.getElementById('progress-title');
    const progressText = document.getElementById('progress-text');
    const progressBar = document.getElementById('progress-bar');
    const cancelBtn = document.getElementById('progress-cancel-btn');

    let pollInterval = setInterval(async () => {
        try {
            const statusResponse = await fetch(`/api/tasks/status?id=${taskId}`);
            if (!statusResponse.ok) throw new Error('Task not found.');

            const progress = await statusResponse.json();

            const percent = progress.total > 0 ? (progress.processed / progress.total) * 100 : 0;
            progressBar.style.width = `${percent}%`;
            progressText.textContent = `Processing ${progress.processed || 0} of ${progress.total || 0}: ${progress.filename || ''}`;

            if (progress.error) throw new Error(progress.error);

            if (progress.complete) {
                clearInterval(pollInterval);
                progressText.textContent = 'Task complete!';
                setTimeout(() => {
                    progressModal.style.display = 'none';
                    if (onComplete) onComplete();
                }, 1500);
            }
        } catch (pollError) {
            clearInterval(pollInterval);
            alert(`An error occurred: ${pollError.message}`);
            progressModal.style.display = 'none';
        }
    }, 750);

    cancelBtn.onclick = async () => {
        clearInterval(pollInterval);
        await fetch(`/api/tasks/cancel?id=${taskId}`, { method: 'POST' });
        progressModal.style.display = 'none';
        if (onCancel) onCancel();
    };
}