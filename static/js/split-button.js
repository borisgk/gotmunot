document.addEventListener('DOMContentLoaded', function() {
    const menuButton = document.getElementById('selection-menu-button');
    const menu = document.getElementById('selection-menu');
    const clearSelectionButton = document.getElementById('clear-selection-btn');
    const addToAlbumItem = document.getElementById('add-to-album-item');

    if (menuButton && menu) {
        menuButton.addEventListener('click', function(event) {
            event.stopPropagation(); // Prevent the window click listener from firing immediately
            const isExpanded = menuButton.getAttribute('aria-expanded') === 'true';
            menu.classList.toggle('show');
            menuButton.setAttribute('aria-expanded', !isExpanded);
        });

        // Close the menu if clicking outside of it
        window.addEventListener('click', function(event) {
            if (!menu.contains(event.target) && !menuButton.contains(event.target)) {
                if (menu.classList.contains('show')) {
                    menu.classList.remove('show');
                    menuButton.setAttribute('aria-expanded', 'false');
                }
            }
        });

        // Reset menu state when selection is cleared
        if (clearSelectionButton) {
            clearSelectionButton.addEventListener('click', function() {
                menu.classList.remove('show');
                menuButton.setAttribute('aria-expanded', 'false');
            });
        }

        // Handle "Add to album..." click
        if (addToAlbumItem) {
            addToAlbumItem.addEventListener('click', function() {
                if (typeof window.openAddToAlbumModal === 'function') {
                    window.openAddToAlbumModal();
                }
                menu.classList.remove('show');
            });
        }
    }
});