document.addEventListener('DOMContentLoaded', () => {
    const form = document.getElementById('new-album-form');
    if (!form) return;

    form.addEventListener('submit', async (event) => {
        event.preventDefault();

        const albumNameInput = document.getElementById('album-name');
        const albumDescriptionInput = document.getElementById('album-description');
        const primaryBtn = form.querySelector('.primary-btn');
        const secondaryBtn = form.querySelector('.secondary-btn');

        const albumName = albumNameInput.value.trim();
        const albumDescription = albumDescriptionInput.value.trim();

        if (!albumName) {
            alert('Album name is required.');
            return;
        }

        // Disable buttons to prevent multiple submissions
        primaryBtn.disabled = true;
        secondaryBtn.disabled = true;

        try {
            const response = await fetch('/api/albums', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name: albumName, description: albumDescription }),
            });

            if (!response.ok) {
                const errorText = await response.text();
                throw new Error(errorText || 'Failed to create album.');
            }

            // On success, redirect to the albums page.
            window.location.href = '/albums';
        } catch (error) {
            alert(`Error: ${error.message}`);
            primaryBtn.disabled = false;
            secondaryBtn.disabled = false;
        }
    });
});