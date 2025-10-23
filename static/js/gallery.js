// Wait for the DOM to be fully loaded before running scripts
document.addEventListener('DOMContentLoaded', (event) => {

    // Get the modal
    const lightbox = document.getElementById("lightbox");
    if (lightbox) {
        const lightboxImg = document.getElementById("lightbox-img");
        const closeBtn = lightbox.querySelector(".close");

        // When the user clicks on <span> (x), close the modal
        closeBtn.onclick = function() {
            lightbox.style.display = "none";
        }
    }

    /* Info Modal Logic */
    const infoModal = document.getElementById("info-modal");
    const infoModalBody = document.getElementById("info-modal-body");
    const infoModalCloseBtn = infoModal.querySelector(".close");

    infoModalCloseBtn.onclick = function() {
        infoModal.style.display = "none";
    }
    window.onclick = function(event) {
        if (event.target == infoModal) {
            infoModal.style.display = "none";
        }
    }

    /* --- Selection & Action Bar Logic --- */
    const selectionBar = document.getElementById('selection-bar');
    const selectionCount = document.getElementById('selection-count');

    function updateSelectionBar() {
        const selectedCheckboxes = document.querySelectorAll('.photo-select-checkbox:checked');
        const count = selectedCheckboxes.length;

        if (count > 0) {
            selectionCount.textContent = `${count} Selected`;
            selectionBar.classList.add('active');
        } else {
            selectionBar.classList.remove('active');
        }
    }

    document.addEventListener('change', function(event) {
        if (event.target.matches('.photo-select-checkbox')) {
            const photoItem = event.target.closest('.photo-item');
            if (event.target.checked) {
                photoItem.classList.add('selected');
            } else {
                photoItem.classList.remove('selected');
            }
            updateSelectionBar();
        }
    });

    document.getElementById('regenerate-selected-btn').addEventListener('click', function(event) {
        event.preventDefault(); // Prevent link navigation
        document.getElementById('selection-dropdown').classList.remove('show');

        const selectedCheckboxes = document.querySelectorAll('.photo-select-checkbox:checked');
        const filenames = Array.from(selectedCheckboxes).map(cb => cb.dataset.filename);

        if (filenames.length === 0) {
            alert('No photos selected for regeneration.');
            return;
        }

        if (confirm(`Start regeneration for ${filenames.length} selected photo(s)? This will happen in the background.`)) {
            fetch('/api/photos/regenerate', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ filenames: filenames }),
            })
            .then(response => {
                if (response.ok) {
                    alert(`Regeneration started for ${filenames.length} photo(s).`);
                } else {
                    alert(`Error starting regeneration: ${response.statusText}`);
                }
            })
            .catch(error => alert('A network error occurred.'));
        }
    });

    document.getElementById('delete-selected-btn').addEventListener('click', function(event) {
        event.preventDefault(); // Prevent link navigation
        document.getElementById('selection-dropdown').classList.remove('show');

        const selectedCheckboxes = document.querySelectorAll('.photo-select-checkbox:checked');
        const filenames = Array.from(selectedCheckboxes).map(cb => cb.dataset.filename);

        if (filenames.length === 0) {
            alert('No photos selected.');
            return;
        }

        if (confirm(`Are you sure you want to delete ${filenames.length} selected photo(s)? This cannot be undone.`)) {
            fetch('/api/photos/delete', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ filenames: filenames }),
            })
            .then(response => {
                if (response.ok) {
                    // On success, remove the items from the DOM and reset the selection bar
                    selectedCheckboxes.forEach(cb => cb.closest('.photo-item').remove());
                    updateSelectionBar();
                } else {
                    alert(`Error deleting photos: ${response.statusText}`);
                }
            })
            .catch(error => alert('A network error occurred.'));
        }
    });

    document.getElementById('clear-selection-btn').addEventListener('click', function(event) {
        event.preventDefault(); // Prevent link navigation
        document.getElementById('selection-dropdown').classList.remove('show');

        // Find all checked boxes and un-check them
        document.querySelectorAll('.photo-select-checkbox:checked').forEach(cb => {
            cb.checked = false;
            cb.closest('.photo-item').classList.remove('selected');
        });

        // Update the bar (which will hide it)
        updateSelectionBar();
    });

    /* Dropdown Menu Logic */
    document.addEventListener('click', function(event) {
        // Close all open dropdowns unless a menu button is clicked
        const selectionMenuTrigger = document.getElementById('selection-menu-trigger');
        const selectionDropdown = document.getElementById('selection-dropdown');

        // If the click is on the selection trigger, toggle its dropdown
        if (selectionMenuTrigger.contains(event.target)) {
            event.preventDefault();
            selectionDropdown.classList.toggle('show');
        } else {
            // Otherwise, if the click is outside the selection dropdown, close it.
            if (selectionDropdown.classList.contains('show') && !selectionDropdown.contains(event.target)) {
                selectionDropdown.classList.remove('show');
            }
        }

        // Close any open per-photo dropdowns if the click was not on their menu button
        if (!event.target.matches('.photo-menu-btn')) {
            document.querySelectorAll('.photo-item .dropdown-content.show').forEach(d => d.classList.remove('show'));
        }

        // When the user clicks anywhere on the lightbox background, close it
        if (event.target === lightbox) {
            lightbox.style.display = "none";
        }

        // Handle lightbox trigger clicks
        if (event.target.closest('.lightbox-trigger')) {
            event.preventDefault();
            lightbox.style.display = "block";
            lightbox.querySelector('#lightbox-img').src = event.target.closest('.lightbox-trigger').dataset.preview;
        }

        // Handle menu button clicks
        if (event.target.matches('.photo-menu-btn')) {
            event.preventDefault();
            // Find the dropdown content relative to the button
            const dropdown = event.target.nextElementSibling;
            dropdown.classList.toggle('show');
        }

        // Handle "Info" button clicks
        if (event.target.matches('.info-btn')) {
            event.preventDefault();
            const filename = event.target.closest('.photo-item').querySelector('.photo-menu-btn').dataset.filename;
            
            fetch(`/photo/info/${filename}`)
                .then(response => {
                    if (!response.ok) {
                        throw new Error(`HTTP error! status: ${response.status}`);
                    }
                    return response.json();
                })
                .then(data => {
                    let content = `<h3>${data.Filename}</h3>`;
                    content += '<table>';
                    
                    // Helper to add a row if the value is valid
                    const addRow = (label, value) => {
                        if (value && value.Valid) { // Check for uppercase 'Valid'
                            let displayValue = value.String || value.Int64 || value.Float64; // Check for uppercase properties
                            if (value.Time) displayValue = new Date(value.Time).toLocaleString(); // Check for uppercase 'Time'
                            content += `<tr><td><strong>${label}</strong></td><td>${displayValue}</td></tr>`;
                        }
                    };

                    addRow('Date Taken', data.DateTimeOriginal);
                    addRow('Dimensions', { Valid: data.ImageWidth.Valid, String: `${data.ImageWidth.Int64} x ${data.ImageLength.Int64}` }); // This custom object is fine
                    addRow('Camera Make', data.Make);
                    addRow('Camera Model', data.Model);
                    addRow('Lens Model', data.LensModel);
                    addRow('F-Number', data.FNumber);
                    addRow('Exposure Time', data.ExposureTime);
                    addRow('ISO', data.ISOSpeedRatings);
                    addRow('Focal Length', { Valid: data.FocalLength.Valid, String: `${data.FocalLength.Float64}mm` }); // This custom object is fine

                    content += '</table>';
                    infoModalBody.innerHTML = content;
                    infoModal.style.display = "block";
                })
                .catch(error => {
                    console.error('Error fetching photo info:', error);
                    alert('Could not load photo information.');
                });
        }

        // Handle "Delete" button clicks
        if (event.target.matches('.delete-btn')) {
            event.preventDefault();
            const photoItem = event.target.closest('.photo-item');
            const filename = photoItem.querySelector('.photo-menu-btn').dataset.filename;
            
            if (confirm(`Are you sure you want to delete "${filename}"? This cannot be undone.`)) {
                fetch(`/api/photo/${filename}`, {
                    method: 'DELETE',
                })
                .then(response => {
                    if (response.ok) {
                        // If deletion is successful, remove the photo item from the DOM
                        photoItem.remove();
                    } else {
                        // If there was an error, alert the user
                        alert(`Error deleting photo: ${response.statusText}`);
                    }
                })
                .catch(error => {
                    console.error('Error during photo deletion:', error);
                    alert('A network error occurred while trying to delete the photo.');
                });
            }
        }
    });

    /* Lazy Loading Logic */
    // The initial photo gallery container has the necessary data attributes.
    // We now use a single parent container for these attributes.
    const galleryContainer = document.getElementById('gallery-container');

    if (galleryContainer) {
        const username = galleryContainer.dataset.username;
        let currentPage = 1;
        const limit = parseInt(galleryContainer.dataset.limit, 10) || 50;
        const filterYear = parseInt(galleryContainer.dataset.filterYear, 10) || 0;
        const totalPhotos = parseInt(galleryContainer.dataset.totalPhotos, 10);
        let loadedPhotosCount = document.querySelectorAll('.photo-item').length;
        let isLoading = false;

        // Helper to get the date part of a timestamp string (YYYY-MM-DD)
        function getDateString(isoString) {
            return isoString.substring(0, 10);
        }

        // Helper to format date string into a readable format
        function formatDate(dateString) {
            const date = new Date(dateString + 'T00:00:00'); // Add time to avoid timezone issues
            return date.toLocaleDateString(undefined, {
                weekday: 'long',
                year: 'numeric',
                month: 'long',
                day: 'numeric',
            });
        }

        // Function to create a photo item from data
        function createPhotoElement(photo) {
            const photoItem = document.createElement('div');
            photoItem.className = 'photo-item';

            // Determine year for the data-year attribute
            let photoYear = new Date(photo.UploadedAt).getFullYear();
            if (photo.DateTimeOriginal && photo.DateTimeOriginal.Valid) {
                photoYear = new Date(photo.DateTimeOriginal.Time).getFullYear();
            } else if (photo.DateTime && photo.DateTime.Valid) {
                photoYear = new Date(photo.DateTime.Time).getFullYear();
            }
            photoItem.dataset.year = photoYear;

            const thumbPath = `/media/${username}/thumbs/${photo.Filepath}.webp`;
            const previewPath = `/media/${username}/previews/${photo.Filepath}`;

            photoItem.innerHTML = `
                <input type="checkbox" class="photo-select-checkbox" data-filename="${photo.Filename}">
                <a href="#" class="lightbox-trigger" data-preview="${previewPath}">
                    <img src="${thumbPath}" alt="${photo.Filename}" title="${photo.Filename}">
                </a>
                <button class="photo-menu-btn" data-filename="${photo.Filename}">&#8942;</button>
                <div class="dropdown-content">
                    <a href="#" class="info-btn">Info</a>
                    <a href="#" class="delete-btn">Delete</a>
                </div>
            `;
            return photoItem;
        }

        // Function to find or create the correct gallery container for a photo
        function getOrCreateGalleryContainer(photo) {
            let photoDate = new Date(photo.UploadedAt);
            if (photo.DateTimeOriginal && photo.DateTimeOriginal.Valid) {
                photoDate = new Date(photo.DateTimeOriginal.Time);
            } else if (photo.DateTime && photo.DateTime.Valid) {
                photoDate = new Date(photo.DateTime.Time);
            }
            const photoDateStr = getDateString(photoDate.toISOString());

            let lastGallery = document.querySelector('.photo-gallery:last-of-type');
            let lastDateSeparator = document.querySelector('.date-separator:last-of-type');
            const lastDateStr = lastDateSeparator.dataset.date;

            if (photoDateStr !== lastDateStr) {
                // Create a new header and gallery div
                const newHeader = document.createElement('h2');
                newHeader.className = 'date-separator';
                newHeader.dataset.date = photoDateStr; // Set the data-date attribute
                newHeader.textContent = formatDate(photoDateStr);
                galleryContainer.appendChild(newHeader);

                const newGallery = document.createElement('div');
                newGallery.className = 'photo-gallery';
                galleryContainer.appendChild(newGallery);
                return newGallery;
            }
            return lastGallery;
        }

        // Function to load more photos
        function loadMorePhotos() {
            // Stop if we are already loading or have loaded all photos
            if (isLoading || loadedPhotosCount >= totalPhotos) {
                return;
            }

            isLoading = true;

            // The offset is the number of photos we already have.
            let apiUrl = `/api/photos?offset=${loadedPhotosCount}&limit=${limit}`;
            if (filterYear > 0) {
                apiUrl += `&year=${filterYear}`;
            }
            
            fetch(apiUrl)
                .then(response => response.json())
                .then(photos => {
                    if (photos && photos.length > 0) {
                        photos.forEach(photo => {
                            const container = getOrCreateGalleryContainer(photo);
                            const photoEl = createPhotoElement(photo);
                            container.appendChild(photoEl);
                        });
                        loadedPhotosCount += photos.length;
                    }
                    isLoading = false;
                })
                .catch(error => {
                    console.error('Error loading more photos:', error);
                    isLoading = false; // Reset on error
                });
        }

        // Listen for scroll events
        window.addEventListener('scroll', () => {
            // Check if user has scrolled to the bottom of the page (with a 250px buffer)
            if ((window.innerHeight + window.scrollY) >= document.body.offsetHeight - 250) {
                loadMorePhotos();
            }
        });
    }

    /* Year Bar Logic */
    const yearBar = document.querySelector('.year-bar');
    if (yearBar) {
        yearBar.addEventListener('click', (event) => {
            if (event.target.matches('.year-link')) {
                // Don't prevent default for the "All" link
                if (event.target.dataset.year) {
                    event.preventDefault();
                    const year = event.target.dataset.year;
                    window.location.href = `/gallery?year=${year}`;
                }
            }
        });
    }
});