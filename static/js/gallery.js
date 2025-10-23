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
            }, 150); // Poll every 0.75 seconds
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