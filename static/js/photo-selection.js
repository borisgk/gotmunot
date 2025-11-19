// This script ensures that when a photo is deselected, its corresponding date checkbox is also deselected.
document.addEventListener('DOMContentLoaded', () => {
    const photoCheckboxes = document.querySelectorAll('.photo-item .photo-select-checkbox');

    photoCheckboxes.forEach(checkbox => {
        checkbox.addEventListener('change', (event) => {
            if (!event.target.checked) {
                // Find the parent .photo-gallery and then its preceding .date-header
                const photoGallery = event.target.closest('.photo-gallery');
                if (photoGallery && photoGallery.previousElementSibling.classList.contains('date-header')) {
                    const dateHeader = photoGallery.previousElementSibling;
                    const dateCheckbox = dateHeader.querySelector('.day-select-checkbox');
                    if (dateCheckbox) dateCheckbox.checked = false;
                }
            }
        });
    });
});