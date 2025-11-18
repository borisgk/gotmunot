// /home/ubuntu/go/src/tm25/static/js/app-bar.js
    const appBar = document.querySelector('.m3-app-bar');
    const navRail = document.querySelector('.m3-nav-rail');
    const body = document.body;
    const normalContent = appBar?.querySelector('.normal-content');
    const contextualContent = appBar?.querySelector('.contextual-content');
    const selectionCountSpan = document.getElementById('selection-count');
    const navRailTrigger = document.getElementById('nav-rail-trigger');

    // Split button elements
    const selectionMenuTrigger = document.getElementById('selection-menu-trigger');
    const selectionMenu = document.getElementById('selection-menu');
    
    // Function to update the bar state
    window.updateSelectionBar = function() {
        const selectedCheckboxes = document.querySelectorAll('.photo-select-checkbox:checked');
        const count = selectedCheckboxes.length;

        if (count > 0) {
            selectionCountSpan.textContent = `${count} selected`;
            appBar.classList.add('contextual');
            normalContent.style.display = 'none';
            contextualContent.style.display = 'flex';
        } else {
            appBar.classList.remove('contextual');
            normalContent.style.display = 'flex';
            contextualContent.style.display = 'none';
        }
    };

    // Navigation Rail Toggle Logic
    if (navRailTrigger && navRail) {
        navRailTrigger.addEventListener('click', () => {
            navRail.classList.toggle('visible');
        });
    }

    // Contextual Menu (Split Button) Logic
    if (selectionMenuTrigger && selectionMenu) {
        selectionMenuTrigger.addEventListener('click', (e) => {
            e.stopPropagation(); // Prevent document click listener from closing it immediately
            selectionMenu.classList.toggle('show');
        });

        // Add to Album action
        document.getElementById('add-to-album-action').addEventListener('click', (e) => {
            e.preventDefault();
            selectionMenu.classList.remove('show');
            document.getElementById('add-to-album-modal').style.display = 'block';
        });

        // Batch Change Date action
        document.getElementById('batch-change-date-action').addEventListener('click', (e) => {
            e.preventDefault();
            selectionMenu.classList.remove('show');
            document.getElementById('batch-change-date-modal').style.display = 'block';
        });
    }

    // Global click listener to close the menu
    document.addEventListener('click', (e) => {
        // Close the selection menu if it's open and the click is outside
        if (selectionMenu && selectionMenu.classList.contains('show') && !e.target.closest('.m3-split-button')) {
            selectionMenu.classList.remove('show');
        }
    });
document.addEventListener('DOMContentLoaded', () => {
    // Add ripple effect to all icon buttons in the app bar
    const iconButtons = document.querySelectorAll('.m3-app-bar .icon-button');

    iconButtons.forEach(button => {
        button.addEventListener('click', function(e) {
            const rect = button.getBoundingClientRect();
            const ripple = document.createElement('span');
            const diameter = Math.max(button.clientWidth, button.clientHeight);
            const radius = diameter / 2;

            ripple.style.width = ripple.style.height = `${diameter}px`;
            ripple.style.left = `${e.clientX - rect.left - radius}px`;
            ripple.style.top = `${e.clientY - rect.top - radius}px`;
            ripple.classList.add('ripple');

            // Remove any existing ripples before adding a new one
            const oldRipple = button.querySelector('.ripple');
            if (oldRipple) {
                oldRipple.remove();
            }

            button.appendChild(ripple);
        });
    });
});