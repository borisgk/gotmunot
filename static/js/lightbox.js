/**
 * Lightbox class to handle image preview and navigation.
 */
class Lightbox {
    constructor(modalId) {
        this.modal = document.getElementById(modalId);
        if (!this.modal) return;

        this.image = this.modal.querySelector('#lightbox-img');
        this.closeBtn = this.modal.querySelector('.close');

        // Toolbar buttons
        this.infoBtn = this.modal.querySelector('#lightbox-info-btn');
        this.downloadBtn = this.modal.querySelector('#lightbox-download-btn');
        this.deleteBtn = this.modal.querySelector('#lightbox-delete-btn');
        this.toolbarCloseBtn = this.modal.querySelector('#lightbox-close-btn');

        this.currentImageUrl = '';

        this.initListeners();
    }

    initListeners() {
        // Close on click of 'x'
        if (this.closeBtn) {
            this.closeBtn.addEventListener('click', () => this.close());
        }

        // Toolbar Close
        if (this.toolbarCloseBtn) {
            this.toolbarCloseBtn.addEventListener('click', () => this.close());
        }

        // Toolbar Info
        if (this.infoBtn) {
            this.infoBtn.addEventListener('click', (e) => {
                e.stopPropagation(); // Prevent closing lightbox
                this.showInfo();
            });
        }

        // Toolbar Download
        if (this.downloadBtn) {
            this.downloadBtn.addEventListener('click', (e) => {
                e.stopPropagation();
                this.downloadImage();
            });
        }

        // Toolbar Delete
        if (this.deleteBtn) {
            this.deleteBtn.addEventListener('click', (e) => {
                e.stopPropagation();
                this.deleteImage();
            });
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
            this.currentImageUrl = url;
            this.image.src = url;
            this.modal.style.display = 'block';
        } catch (error) {
            console.error('Error opening lightbox:', error);
        }
    }

    close() {
        this.modal.style.display = 'none';
        this.image.src = ''; // Clear source
        this.currentImageUrl = '';
    }

    showInfo() {
        // Assuming showPhotoInfo is a global function or available in gallery.js scope
        // We need to extract the filename from the URL or store it.
        // URL format: /media/user/originals/YYYY/MM/DD/filename.jpg
        if (!this.currentImageUrl) return;

        const filename = this.currentImageUrl.split('/').pop();
        if (typeof showPhotoInfo === 'function') {
            showPhotoInfo(filename);
        } else {
            console.warn('showPhotoInfo function not found');
        }
    }

    downloadImage() {
        if (!this.currentImageUrl) return;

        const link = document.createElement('a');
        link.href = this.currentImageUrl;
        link.download = this.currentImageUrl.split('/').pop();
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
    }

    deleteImage() {
        if (!this.currentImageUrl) return;
        const filename = this.currentImageUrl.split('/').pop();

        if (confirm('Are you sure you want to delete this photo?')) {
            fetch(`/api/photo/${filename}`, {
                method: 'DELETE',
            })
                .then(response => {
                    if (response.ok) {
                        this.close();
                        // Reload or remove element from grid
                        window.location.reload();
                    } else {
                        alert('Failed to delete photo.');
                    }
                })
                .catch(error => {
                    console.error('Error deleting photo:', error);
                    alert('Error deleting photo.');
                });
        }
    }
}
