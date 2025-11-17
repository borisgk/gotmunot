// /home/ubuntu/go/src/tm25/static/js/app-bar.js
document.addEventListener('DOMContentLoaded', () => {
    const appBar = document.querySelector('.m3-app-bar');
    const navRail = document.querySelector('.m3-nav-rail');
    const body = document.body;
    const normalContent = appBar?.querySelector('.normal-content');
    const contextualContent = appBar?.querySelector('.contextual-content');
    const selectionCountSpan = document.getElementById('selection-count');
    const navRailTrigger = document.getElementById('nav-rail-trigger');
    
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