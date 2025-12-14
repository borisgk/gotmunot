// Generic task poller to be used across different pages.
// It requires a modal with the following element IDs to be present in the HTML:
// - progress-modal
// - progress-title
// - progress-text
// - progress-bar
// - progress-cancel-btn

window.pollTaskStatus = function (taskId, { onComplete, onCancel } = {}) {
    const progressModal = document.getElementById('progress-modal');
    const progressTitle = document.getElementById('progress-title');
    const progressText = document.getElementById('progress-text');
    const progressBar = document.getElementById('progress-bar');
    const cancelBtn = document.getElementById('progress-cancel-btn');
    let displayedThumbCount = 0; // Keep track of how many thumbs we've shown

    let pollInterval = setInterval(async () => {
        try {
            const statusResponse = await fetch(`/api/tasks/status?id=${taskId}`);
            if (!statusResponse.ok) throw new Error('Task not found.');

            const progress = await statusResponse.json();

            const percent = progress.total > 0 ? (progress.processed / progress.total) * 100 : 0;
            progressBar.style.width = `${percent}%`;
            progressText.textContent = `Processing ${progress.processed || 0} of ${progress.total || 0}: ${progress.filename || ''}`;

            // Check for new thumbnails to display
            if (progress.generated_thumbnails && progress.generated_thumbnails.length > displayedThumbCount) {
                const newThumbs = progress.generated_thumbnails.slice(displayedThumbCount);
                if (typeof window.addThumbnailToGrid === 'function') {
                    newThumbs.forEach(thumbUrl => window.addThumbnailToGrid(thumbUrl));
                }
                displayedThumbCount = progress.generated_thumbnails.length;
            }

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
            console.error(`An error occurred: ${pollError.message}`);
            progressText.textContent = `Error: ${pollError.message}`;
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