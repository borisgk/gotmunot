// Wait for the DOM to be fully loaded before running scripts

let snackbarTimeout;
/**
 * Shows a Material Design 3 snackbar with a message.
 * @param {string} message The message to display.
 */
function showSnackbar(message) {
    const snackbar = document.getElementById('m3-snackbar');
    const label = snackbar.querySelector('.m3-snackbar-label');
    if (!snackbar || !label) return;

    clearTimeout(snackbarTimeout);
    label.textContent = message;
    snackbar.classList.add('show');
    snackbarTimeout = setTimeout(() => snackbar.classList.remove('show'), 5000);
}

document.addEventListener('DOMContentLoaded', (event) => {

    // Get all modals at the top level of the DOMContentLoaded listener
    // Initialize Lightbox
    const lightbox = new Lightbox("lightbox");

    const infoModal = document.getElementById("info-modal");
    const changeDateModal = document.getElementById('change-date-modal');

    // Handle authentication required event
    document.addEventListener('auth:required', () => {
        const loginModal = document.getElementById('login-modal');
        const loginForm = document.getElementById('modal-login-form');
        const errorP = document.getElementById('login-modal-error');

        if (!loginModal || !loginForm) return;

        errorP.textContent = ''; // Clear previous errors
        loginModal.style.display = 'block';

        loginForm.onsubmit = async (e) => {
            e.preventDefault();
            const formData = new FormData(loginForm);
            try {
                const response = await fetch('/api/login', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    credentials: 'include',
                    body: JSON.stringify(Object.fromEntries(formData)),
                });
                if (response.ok) {
                    loginModal.style.display = 'none';
                    // Optionally reload or retry the action
                    window.location.reload();
                } else {
                    errorP.textContent = 'Login failed. Please try again.';
                }
            } catch (error) {
                console.error('Login error:', error);
                errorP.textContent = 'An error occurred during login.';
            }
        };
    });

    /* --- Change Date Modal Logic --- */
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

    /* --- Add to Album Modal Logic --- */
    const addToAlbumModal = document.getElementById('add-to-album-modal');
    if (addToAlbumModal) {
        const closeBtn = addToAlbumModal.querySelector('.close');
        const cancelBtn = document.getElementById('add-to-album-cancel-btn');
        const form = document.getElementById('add-to-album-form');
        const albumSelect = document.getElementById('album-select');

        // Function to fetch albums and populate the dropdown
        async function populateAlbumSelect() {
            try {
                const response = await fetch('/api/albums/list');
                if (response.status === 401) {
                    // Session expired, show login modal.
                    const loginModal = document.getElementById('login-modal');
                    if (loginModal) loginModal.style.display = 'block';
                    throw new Error('User not authenticated.');
                }
                if (!response.ok) throw new Error('Failed to fetch albums.');
                const albums = await response.json();

                albumSelect.innerHTML = ''; // Clear existing options

                if (!albums || albums.length === 0) {
                    albumSelect.innerHTML = '<option value="">No albums found</option>';
                    return;
                }

                albums.forEach(album => {
                    const option = new Option(album.name, album.id);
                    albumSelect.add(option);
                });
            } catch (error) {
                console.error('Error populating albums:', error);
                albumSelect.innerHTML = '<option value="">Error loading albums</option>';
            }
        }

        const closeAddToAlbumModal = () => {
            addToAlbumModal.classList.remove('show');
        };

        cancelBtn.onclick = closeAddToAlbumModal;

        // Expose a global function to open the modal and populate it
        window.openAddToAlbumModal = async function () {
            await populateAlbumSelect();
            addToAlbumModal.classList.add('show');
            albumSelect.focus(); // For better UX
        };

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
                showSnackbar(`${result.photos_added} new photo(s) added to the album.`);
                closeAddToAlbumModal();
                document.getElementById('clear-selection-btn').click(); // Clear selection
            } catch (error) {
                alert(`Error: ${error.message}`);
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
    document.addEventListener('change', function (event) {
        if (event.target.matches('.photo-select-checkbox')) {
            const photoItem = event.target.closest('.photo-item');
            if (event.target.checked) {
                photoItem.classList.add('selected');
            } else {
                photoItem.classList.remove('selected');
            }
            // Call the function to update the app bar's state
            window.updateSelectionBar();
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

    document.getElementById('clear-selection-btn').addEventListener('click', function (event) {
        event.preventDefault(); // Prevent link navigation

        // Find all checked boxes and un-check them
        document.querySelectorAll('.photo-select-checkbox:checked').forEach(cb => {
            cb.checked = false;
            cb.closest('.photo-item').classList.remove('selected');
        });

        // Update the bar (which will hide it)
        updateSelectionBar();
    });

    // Consolidated click handler for the entire document
    document.addEventListener('click', function (event) {

        // Handle lightbox trigger clicks
        if (event.target.closest('.lightbox-trigger')) {
            event.preventDefault();
            const previewUrl = event.target.closest('.lightbox-trigger').dataset.preview;
            lightbox.open(previewUrl);
        }

        // Close info modal
        if (infoModal) {
            if (event.target === infoModal) {
                infoModal.classList.remove('show');
            }
        }

        // Handle Info Modal Close Button
        if (event.target.matches('#info-modal-close-btn')) {
            if (infoModal) infoModal.classList.remove('show');
        }

        // Handle menu button clicks
        const menuBtn = event.target.closest('.photo-menu-btn');

        if (menuBtn) {
            event.preventDefault();
            const dropdown = menuBtn.nextElementSibling;

            // Close other open dropdowns
            document.querySelectorAll('.photo-item .dropdown-content.show').forEach(d => {
                if (d !== dropdown) {
                    d.classList.remove('show');
                }
            });

            if (dropdown) {
                dropdown.classList.toggle('show');
            }
        } else {
            // Close any open per-photo dropdowns if the click was not on a menu button
            document.querySelectorAll('.photo-item .dropdown-content.show').forEach(d => d.classList.remove('show'));
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
                    // Clear previous content
                    infoModalBody.innerHTML = '';

                    const table = document.createElement('table');
                    table.style.width = '100%';
                    table.style.borderCollapse = 'collapse';

                    const addStyledRow = (label, value) => {
                        const row = table.insertRow();
                        const cell1 = row.insertCell(0);
                        const cell2 = row.insertCell(1);

                        cell1.innerHTML = `<strong>${label}</strong>`;
                        cell1.style.padding = '8px 0';
                        cell1.style.borderBottom = '1px solid #eee';
                        cell1.style.color = 'var(--m3-on-surface-variant)';

                        cell2.textContent = value;
                        cell2.style.padding = '8px 0';
                        cell2.style.borderBottom = '1px solid #eee';
                        cell2.style.textAlign = 'right';
                        cell2.style.color = 'var(--m3-on-surface)';
                    };

                    addStyledRow('Filename', data.Filename);
                    addStyledRow('ID', data.ID);
                    addStyledRow('Date Taken', data.DateTime ? new Date(data.DateTime).toLocaleString() : 'N/A');
                    addStyledRow('Uploaded At', data.UploadedAt ? new Date(data.UploadedAt).toLocaleString() : 'N/A');
                    addStyledRow('Original Size', `${data.ImageWidth} x ${data.ImageLength}`);
                    addStyledRow('Preview Size', `${data.PreviewWidth} x ${data.PreviewHeight}`);
                    addStyledRow('Thumbnail Size', `${data.ThumbWidth} x ${data.ThumbHeight}`);
                    addStyledRow('File Path', data.Filepath);
                    addStyledRow('Uploaded By', data.UploadedBy);

                    infoModalBody.appendChild(table);
                    infoModal.classList.add('show');
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