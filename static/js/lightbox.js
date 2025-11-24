/**
 * Lightbox class to handle image preview and navigation.
 */
class Lightbox {
    constructor(modalId) {
        this.modal = document.getElementById(modalId);
        if (!this.modal) return;

        this.image = this.modal.querySelector('#lightbox-img');
        this.closeBtn = this.modal.querySelector('.close');

        this.initListeners();
    }

    initListeners() {
        // Close on click of 'x'
        if (this.closeBtn) {
            this.closeBtn.addEventListener('click', () => this.close());
        }

        // Close on click outside the image
        this.modal.addEventListener('click', (e) => {
            if (e.target === this.modal) {
                this.close();
            }
        });

        // Close on Escape key
        document.addEventListener('keydown', (e) => {
            if (e.key === 'Escape' && this.isOpen()) {
                this.close();
            }
        });
    }

    isOpen() {
        return this.modal.style.display === 'block';
    }

    async open(url) {
        // Check for auth before showing (logic moved from gallery.js)
        try {
            const response = await fetch(url, { redirect: 'manual' });
            if (response.status === 401) {
                // Dispatch event for main app to handle login
                document.dispatchEvent(new CustomEvent('auth:required'));
                return;
            }

            // If ok, show image
            this.image.src = url;
            this.modal.style.display = 'block';
        } catch (error) {
            console.error('Error opening lightbox:', error);
        }
    }

    close() {
        this.modal.style.display = 'none';
        this.image.src = ''; // Clear source
    }
}
