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

    /* --- Change Date Modal Logic --- */
    const changeDateModal = document.getElementById('change-date-modal');
    if (changeDateModal) {
        const changeDateModalCloseBtn = changeDateModal.querySelector('.close');
        const changeDateCancelBtn = document.getElementById('change-date-cancel-btn');
        const changeDateForm = document.getElementById('change-date-form');

        // Function to close the modal
        const closeChangeDateModal = () => {
            changeDateModal.style.display = 'none';
        };

        changeDateModalCloseBtn.onclick = closeChangeDateModal;
        changeDateCancelBtn.onclick = closeChangeDateModal;

        // Placeholder for form submission
        changeDateForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            const filename = document.getElementById('change-date-filename').value;
            const newDate = document.getElementById('new-date-input').value;

            try {
                const response = await fetch('/api/photo/update-date', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        filename: filename,
                        new_date: newDate,
                    }),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    throw new Error(`Failed to update date: ${errorText}`);
                }

                // On success, close the modal and reload the page to see the change.
                closeChangeDateModal();
                window.location.reload();

            } catch (error) {
                alert(error.message);
            }
        });
    }

    /* --- Batch Change Date Modal Logic --- */
    const batchChangeDateModal = document.getElementById('batch-change-date-modal');
    if (batchChangeDateModal) {
        const closeBtn = batchChangeDateModal.querySelector('.close');
        const cancelBtn = document.getElementById('batch-change-date-cancel-btn');
        const form = document.getElementById('batch-change-date-form');

        const closeBatchChangeDateModal = () => {
            batchChangeDateModal.style.display = 'none';
        };

        closeBtn.onclick = closeBatchChangeDateModal;
        cancelBtn.onclick = closeBatchChangeDateModal;

        form.addEventListener('submit', async (e) => {
            e.preventDefault();
            closeBatchChangeDateModal(); // Close the date input modal first

            const selectedCheckboxes = document.querySelectorAll('.photo-select-checkbox:checked');
            const filenames = Array.from(selectedCheckboxes).map(cb => cb.dataset.filename);
            const newStartDate = document.getElementById('batch-new-date-input').value;

            if (filenames.length === 0) return;

            // Show and manage the progress modal
            await showProgressModalForBatchDateChange(filenames, newStartDate);
        });
    }

    document.getElementById('batch-change-date-btn').addEventListener('click', (e) => {
        e.preventDefault();
        document.getElementById('selection-dropdown').classList.remove('show');
        const selectedCount = document.querySelectorAll('.photo-select-checkbox:checked').length;
        if (selectedCount === 0) {
            alert('Please select photos first.');
            return;
        }
        // Pre-fill with current time
        const now = new Date();
        const timezoneOffset = now.getTimezoneOffset() * 60000;
        const localISOTime = new Date(now.getTime() - timezoneOffset).toISOString().slice(0, 16);
        document.getElementById('batch-new-date-input').value = localISOTime;

        batchChangeDateModal.style.display = 'block';
    });

    /* --- Add to Album Modal Logic --- */
    const addToAlbumModal = document.getElementById('add-to-album-modal');
    if (addToAlbumModal) {
        const closeBtn = addToAlbumModal.querySelector('.close');
        const cancelBtn = document.getElementById('add-to-album-cancel-btn');
        const form = document.getElementById('add-to-album-form');
        const albumSelect = document.getElementById('album-select');

        const closeAddToAlbumModal = () => {
            addToAlbumModal.style.display = 'none';
        };

        closeBtn.onclick = closeAddToAlbumModal;
        cancelBtn.onclick = closeAddToAlbumModal;

        // The form submission logic will be added in a future step.
        form.addEventListener('submit', async (e) => {
            e.preventDefault();
            const selectedAlbumId = albumSelect.value;
            const selectedCheckboxes = document.querySelectorAll('.photo-select-checkbox:checked');
            const filenames = Array.from(selectedCheckboxes).map(cb => cb.dataset.filename);

            if (!selectedAlbumId || filenames.length === 0) {
                alert('Please select an album and at least one photo.');
                return;
            }

            try {
                const response = await fetch('/api/albums/add-photos', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        album_id: parseInt(selectedAlbumId, 10),
                        filenames: filenames,
                    }),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    throw new Error(errorText || 'Failed to add photos to album.');
                }

                const result = await response.json();
                alert(`${result.photos_added} new photo(s) added to the album.`);
                closeAddToAlbumModal();
                document.getElementById('clear-selection-btn').click(); // Clear selection
            } catch (error) {
                alert(`Error: ${error.message}`);
            }
        });

        document.getElementById('add-selected-to-album-btn').addEventListener('click', async (e) => {
            e.preventDefault();
            document.getElementById('selection-dropdown').classList.remove('show');
            const selectedCount = document.querySelectorAll('.photo-select-checkbox:checked').length;
            if (selectedCount === 0) {
                alert('Please select photos first.');
                return;
            }

            // Fetch the list of albums and populate the select dropdown
            try {
                const response = await fetch('/api/albums/list');
                if (!response.ok) throw new Error('Failed to fetch albums.');
                const albums = await response.json();
                albumSelect.innerHTML = albums.map(album => `<option value="${album.id}">${album.name}</option>`).join('');
                addToAlbumModal.style.display = 'block';
            } catch (error) {
                alert(error.message);
            }
        });
    }

    async function showProgressModalForBatchDateChange(filenames, startDate) {
        const progressModal = document.getElementById('progress-modal');
        const progressTitle = document.getElementById('progress-title');
        const progressText = document.getElementById('progress-text');
        const progressBar = document.getElementById('progress-bar');

        progressTitle.textContent = 'Changing Dates...';
        progressText.textContent = 'Starting batch update...';
        progressBar.style.width = '0%';
        progressModal.style.display = 'block';

        try {
            const response = await fetch('/api/photos/batch-update-date', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ filenames: filenames, start_date: startDate }),
            });

            if (!response.ok) throw new Error('Failed to start batch update process.');

            const { task_id } = await response.json();
            
            // Poll for status using a generic polling function
            pollTaskStatus(task_id, {
                onComplete: () => window.location.reload(),
            });
        } catch (error) {
            alert(`Could not start batch update: ${error.message}`);
            progressModal.style.display = 'none';
        }
    }

    function updateTotalCounts(count) {
        // Update the main gallery total count.
        const galleryContainer = document.getElementById('gallery-container');
        if (galleryContainer) {
            const currentTotal = parseInt(galleryContainer.dataset.totalPhotos, 10);
            const newTotal = Math.max(0, currentTotal - count);
            galleryContainer.dataset.totalPhotos = newTotal;
        }

        // Update the "All" photos count in the year bar.
        const allLink = document.querySelector('.year-bar .year-link[href="/gallery"] sup');
        if (allLink) {
            const currentAllCount = parseInt(allLink.textContent, 10);
            const newAllCount = Math.max(0, currentAllCount - count);
            allLink.textContent = newAllCount;
        }

        // Update the count for the currently filtered year, if any.
        const currentYear = galleryContainer.dataset.filterYear;
        if (currentYear && currentYear !== "0") {
            const yearLink = document.querySelector(`.year-bar .year-link[data-year="${currentYear}"] sup`);
            if (yearLink) yearLink.textContent = Math.max(0, parseInt(yearLink.textContent, 10) - count);
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

        // Handle "select all for day" checkbox clicks
        if (event.target.matches('.day-select-checkbox')) {
            const header = event.target.closest('.date-header');
            // The photo gallery for this day is the next sibling element
            const gallery = header.nextElementSibling;

            if (gallery && gallery.classList.contains('photo-gallery')) {
                const shouldBeChecked = event.target.checked;
                gallery.querySelectorAll('.photo-select-checkbox').forEach(cb => {
                    // Only trigger a change if the state is different to avoid extra work
                    if (cb.checked !== shouldBeChecked) {
                        cb.checked = shouldBeChecked;
                        // Manually trigger a change event to update the visual style and selection bar
                        cb.dispatchEvent(new Event('change', { bubbles: true }));
                    }
                });
            }
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
                    const galleriesToUpdate = new Set();
                    selectedCheckboxes.forEach(cb => {
                        const photoItem = cb.closest('.photo-item');
                        if (photoItem) {
                            galleriesToUpdate.add(photoItem.parentElement);
                            photoItem.remove();
                        }
                    });

                    // Check any galleries that were modified to see if they are now empty.
                    galleriesToUpdate.forEach(gallery => {
                        if (gallery && gallery.children.length === 0) {
                            gallery.previousElementSibling?.remove(); // Remove date header
                            gallery.remove();
                        }
                    });
                    updateSelectionBar();
                    updateTotalCounts(filenames.length);
                } else {
                    alert(`Error deleting photos: ${response.statusText}`);
                }
            })
            .catch(error => alert('A network error occurred.'));
        }
    });

    document.getElementById('download-previews-btn').addEventListener('click', function(event) {
        event.preventDefault(); // Prevent link navigation
        document.getElementById('selection-dropdown').classList.remove('show');

        const selectedCheckboxes = document.querySelectorAll('.photo-select-checkbox:checked');
        const filenames = Array.from(selectedCheckboxes).map(cb => cb.dataset.filename);

        if (filenames.length === 0) {
            alert('No photos selected for download.');
            return;
        }

        // Construct the URL with filenames as query parameters.
        // This is safe for a reasonable number of files.
        const query = new URLSearchParams();
        filenames.forEach(name => query.append('filename', name));
        
        // Redirect the browser to trigger the download.
        window.location.href = `/api/photos/download-previews?${query.toString()}`;
    });

    document.getElementById('download-originals-btn').addEventListener('click', async function(event) {
        event.preventDefault(); // Prevent link navigation
        document.getElementById('selection-dropdown').classList.remove('show');

        const selectedCheckboxes = document.querySelectorAll('.photo-select-checkbox:checked');
        const filenames = Array.from(selectedCheckboxes).map(cb => cb.dataset.filename);

        if (filenames.length === 0) { return; }

        // Show the progress modal
        const progressModal = document.getElementById('progress-modal');
        const progressTitle = document.getElementById('progress-title');
        const progressText = document.getElementById('progress-text');
        const progressBar = document.getElementById('progress-bar');
        const cancelBtn = document.getElementById('progress-cancel-btn');

        progressTitle.textContent = 'Preparing Download...';
        progressText.textContent = 'Starting zip process...';
        progressBar.style.width = '0%';
        progressModal.style.display = 'block';

        try {
            let pollInterval; // Define here to be accessible in the cancel handler
            let isCancelled = false;

            // 1. Start the download task
            const startResponse = await fetch('/api/downloads/start', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ filenames: filenames, type: 'originals' }),
            });

            if (!startResponse.ok) {
                throw new Error('Failed to start download process.');
            }

            const { task_id } = await startResponse.json();

            // Handle cancellation
            cancelBtn.onclick = async () => {
                isCancelled = true;
                clearInterval(pollInterval);
                await fetch(`/api/downloads/cancel?id=${task_id}`, { method: 'POST' });
                progressModal.style.display = 'none';
            };

            // 2. Poll for status
            pollInterval = setInterval(async () => {
                // Stop polling if the user has cancelled
                if (isCancelled) {
                    clearInterval(pollInterval);
                    return;
                }
                try {
                    const statusResponse = await fetch(`/api/downloads/status?id=${task_id}`);
                    if (!statusResponse.ok) {
                        // Stop polling if task not found (e.g., server restarted)
                        throw new Error('Task not found.');
                    }

                    const progress = await statusResponse.json();

                    // Update modal UI
                    const percent = progress.total > 0 ? (progress.processed / progress.total) * 100 : 0;
                    progressBar.style.width = `${percent}%`;
                    progressText.textContent = `Processing ${progress.processed || 0} of ${progress.total || 0}: ${progress.filename || ''}`;

                    if (progress.error) {
                        throw new Error(progress.error);
                    }

                    // 3. When complete, trigger download and close modal
                    if (progress.complete && progress.download_url && !progress.cancelled) {
                        clearInterval(pollInterval);
                        progressTitle.textContent = 'Download Ready!';
                        progressText.textContent = 'Your download will begin shortly...';
                        window.location.href = progress.download_url;
                        // Close modal after a short delay
                        setTimeout(() => {
                            progressModal.style.display = 'none';
                        }, 3000);
                    }
                } catch (pollError) {
                    clearInterval(pollInterval);
                    alert(`An error occurred: ${pollError.message}`);
                    progressModal.style.display = 'none';
                }
            }, 750); // Poll every 0.75 seconds
        } catch (startError) {
            alert(`Could not start download: ${startError.message}`);
            progressModal.style.display = 'none';
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
            const previewUrl = event.target.closest('.lightbox-trigger').dataset.preview;

            // Fetch the image first to check for redirects (e.g., to login page)
            fetch(previewUrl, { redirect: 'manual' }) // Important: 'manual' prevents auto-following redirects
                .then(response => {
                    // A response type of 'opaqueredirect' indicates a cross-origin redirect,
                    // which we can't inspect further. A 401 status is a clearer signal.
                    if (response.status === 401) {
                        // The session has expired. Show the login modal.
                        const loginModal = document.getElementById('login-modal');
                        const loginForm = document.getElementById('modal-login-form');
                        const errorP = document.getElementById('login-modal-error');
                        errorP.textContent = ''; // Clear previous errors
                        loginModal.style.display = 'block';

                        loginForm.onsubmit = async (e) => {
                            e.preventDefault();
                            const formData = new FormData(loginForm);
                            const response = await fetch('/api/login', {
                                method: 'POST',
                                headers: { 'Content-Type': 'application/json' },
                                credentials: 'include', // Important: This tells fetch to handle cookies
                                body: JSON.stringify(Object.fromEntries(formData)),
                            });
                            if (response.ok) {
                                loginModal.style.display = 'none';
                            } else {
                                errorP.textContent = 'Login failed. Please try again.';
                            }
                        };
                    } else {
                        // If the response is OK, show the lightbox.
                        lightbox.style.display = "block";
                        lightbox.querySelector('#lightbox-img').src = previewUrl;
                    }
                })
                .catch(error => console.error('Error checking preview URL:', error));
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
            // Hide the dropdown menu
            event.target.closest('.dropdown-content').classList.remove('show');

            const filename = event.target.closest('.photo-item').querySelector('.photo-menu-btn').dataset.filename;

            const infoModal = document.getElementById("info-modal");
            const infoModalBody = document.getElementById("info-modal-body");

            // Function to fetch and display photo info
            async function showPhotoInfo(filename) {
                try {
                    const response = await fetch(`/photo/info/${filename}`);
                    if (!response.ok) {
                        throw new Error(`HTTP error! status: ${response.status}`);
                    }
                    const data = await response.json();

                    // Helper to add a row to the table if the value is not a zero-value
                    const addRow = (table, label, value) => {
                        let displayValue = value;
                        // Format date strings for better readability, but ignore zero-value dates
                        if (label === 'Date Taken' && typeof value === 'string' && value.includes('T') && value.includes('Z') && !value.startsWith('0001-01-01')) {
                            displayValue = new Date(value).toLocaleString('en-US', {
                                year: 'numeric', month: 'long', day: 'numeric',
                                hour: '2-digit', minute: '2-digit',
                                hour12: false // Use 24-hour format
                            });
                        }
                        const row = table.insertRow();
                        row.insertCell(0).innerHTML = `<strong>${label}</strong>`;
                        row.insertCell(1).textContent = displayValue;
                    };
                    // Build the content
                    infoModalBody.innerHTML = `<h3>${data.Filename}</h3>`;
                    const table = document.createElement('table');
                    addRow(table, 'Date Taken', data.DateTime);
                    addRow(table, 'Dimensions', `${data.ImageWidth} x ${data.ImageLength}`);
                    infoModalBody.appendChild(table);
                    infoModal.style.display = "block";
                } catch (error) {
                    console.error('Error fetching photo info:', error);
                    alert('Could not load photo information.');
                }
            }

            // Call the function
            showPhotoInfo(filename);
        }

        // Handle "Change date" button clicks
        if (event.target.matches('.change-date-btn')) {
            event.preventDefault();
            // Hide the dropdown menu
            const dropdown = event.target.closest('.dropdown-content');
            if (dropdown) dropdown.classList.remove('show');

            const filename = event.target.closest('.photo-item').querySelector('.photo-menu-btn').dataset.filename;

            // Fetch current date and open modal
            openChangeDateModal(filename);
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
                        const gallery = photoItem.parentElement;
                        photoItem.remove();
                        updateTotalCounts(1);

                        // Check if the gallery is now empty
                        if (gallery && gallery.children.length === 0) {
                            // If empty, remove the gallery and its corresponding date header
                            const dateHeader = gallery.previousElementSibling;
                            dateHeader?.remove();
                            gallery.remove();
                        }
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

    async function openChangeDateModal(filename) {
        const modal = document.getElementById('change-date-modal');
        const currentDateDisplay = document.getElementById('current-date-display');
        const newDateInput = document.getElementById('new-date-input');
        const filenameInput = document.getElementById('change-date-filename');

        try {
            const response = await fetch(`/photo/info/${filename}`);
            if (!response.ok) throw new Error('Failed to fetch photo info');
            const data = await response.json();

            // Determine the best available date
            // DateTime is now the pre-calculated best date, so we use it directly.
            let currentDate = new Date(data.DateTime);

            // Format for display
            currentDateDisplay.textContent = currentDate.toLocaleString('en-US', {
                year: 'numeric', month: 'long', day: 'numeric',
                hour: '2-digit', minute: '2-digit',
                hour12: false // Use 24-hour format
            });

            // Format for the datetime-local input (YYYY-MM-DDTHH:MM)
            // We need to adjust for the timezone offset to pre-fill correctly.
            const timezoneOffset = currentDate.getTimezoneOffset() * 60000; //offset in milliseconds
            const localISOTime = new Date(currentDate.getTime() - timezoneOffset).toISOString().slice(0, 16);
            newDateInput.value = localISOTime;

            filenameInput.value = filename;
            modal.style.display = 'block';

        } catch (error) {
            console.error('Error opening change date modal:', error);
            alert('Could not load photo information to change date.');
        }
    }
});