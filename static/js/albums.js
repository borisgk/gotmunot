document.addEventListener('DOMContentLoaded', function() {
    const modal = document.getElementById('renameAlbumModal');
    const modalForm = document.getElementById('renameAlbumForm');
    const modalAlbumId = document.getElementById('modalAlbumId');
    const modalAlbumName = document.getElementById('modalAlbumName');
    const modalAlbumDescription = document.getElementById('modalAlbumDescription');
    const cancelBtn = document.getElementById('modalCancelBtn');
    const okBtn = document.getElementById('modalOkBtn');

    const deleteModal = document.getElementById('deleteConfirmModal');
    const deleteCancelBtn = document.getElementById('deleteCancelBtn');
    const deleteConfirmBtn = document.getElementById('deleteConfirmBtn');
    let albumIdToDelete = null;

    document.querySelectorAll('.album-card').forEach(card => {
        const menuIcon = card.querySelector('.album-context-menu');
        const dropdownMenu = card.querySelector('.album-dropdown-menu');

        menuIcon.addEventListener('click', function(event) {
            event.preventDefault(); // Prevent navigating to album page
            event.stopPropagation(); // Stop click from reaching the document listener immediately

            // Hide all other menus before showing this one
            document.querySelectorAll('.album-dropdown-menu').forEach(menu => {
                if (menu !== dropdownMenu) {
                    menu.style.display = 'none';
                }
            });

            dropdownMenu.style.display = dropdownMenu.style.display === 'block' ? 'none' : 'block';
        });
    });

    // --- Modal Logic ---

    // Show modal when "Rename album..." is clicked
    document.querySelectorAll('.rename-album-btn').forEach(btn => {
        btn.addEventListener('click', function(event) {
            event.preventDefault(); // Prevent the parent <a> tag from navigating
            event.stopPropagation(); // Prevent document click listener from firing
            // Hide the context menu that was just clicked
            this.closest('.album-dropdown-menu').style.display = 'none';

            const albumId = this.dataset.albumId;
            const albumName = this.dataset.albumName;
            const albumDescription = this.dataset.albumDescription;

            // Populate and show the modal
            modalAlbumId.value = albumId;
            modalAlbumName.value = albumName;
            modalAlbumDescription.value = albumDescription;
            modal.style.display = 'flex';
        });
    });

    // Close modal on Cancel
    function closeModal() {
        modal.style.display = 'none';
    }
    cancelBtn.addEventListener('click', closeModal);

    // --- Delete Confirmation Modal Logic ---
    document.querySelectorAll('.delete-album-btn').forEach(btn => {
        btn.addEventListener('click', function(event) {
            event.preventDefault();
            event.stopPropagation();
            this.closest('.album-dropdown-menu').style.display = 'none';

            albumIdToDelete = this.dataset.albumId;
            deleteModal.style.display = 'flex';
        });
    });

    function closeDeleteModal() {
        deleteModal.style.display = 'none';
        albumIdToDelete = null;
    }

    deleteCancelBtn.addEventListener('click', closeDeleteModal);

    deleteConfirmBtn.addEventListener('click', function() {
        if (!albumIdToDelete) return;

        fetch(`/api/album/${albumIdToDelete}`, {
            method: 'DELETE',
        }).then(response => {
            if (!response.ok) {
                throw new Error('Failed to delete album');
            }
            // Remove the album card from the DOM
            document.querySelector(`.album-card[href='/album/${albumIdToDelete}']`).remove();
            closeDeleteModal();
        }).catch(error => {
            console.error('Error:', error);
            closeDeleteModal();
        });
    });

    // Handle form submission (OK button)
    okBtn.addEventListener('click', function() {
        // Manually trigger form validation
        if (!modalForm.checkValidity()) {
            modalForm.reportValidity();
            return;
        }

        const albumId = modalAlbumId.value;
        const newName = modalAlbumName.value;
        const newDescription = modalAlbumDescription.value;

        fetch(`/api/album/${albumId}`, {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name: newName, description: newDescription })
        }).then(response => {
            if (!response.ok) throw new Error('Failed to update album');
            return response.json();
        }).then(data => {
            // Update album card on the page
            const albumCard = document.querySelector(`.album-card[href='/album/${albumId}']`);
            if (albumCard) {
                albumCard.querySelector('.album-info h3').textContent = newName;
                albumCard.querySelector('.rename-album-btn').dataset.albumName = newName;
                albumCard.querySelector('.rename-album-btn').dataset.albumDescription = newDescription;
            }
            closeModal();
        }).catch(error => console.error('Error:', error));
    });

    // Hide menu when clicking anywhere else on the page
    document.addEventListener('click', () => document.querySelectorAll('.album-dropdown-menu').forEach(menu => menu.style.display = 'none'));
});