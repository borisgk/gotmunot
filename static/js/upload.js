const uploadForm = document.getElementById('uploadForm');
const fileInput = document.getElementById('photo');
const uploadButton = document.getElementById('uploadButton');
const uploadStatus = document.getElementById('upload-status');
const galleryButton = document.getElementById('galleryButton');
const uploadedFilenames = [];

// Global state for progress
let totalBytes = 0;
let loadedBytesMap = new Map(); // file -> bytesLoaded
let activeFiles = new Set();

function updateGlobalProgress() {
    let totalLoaded = 0;
    loadedBytesMap.forEach((loaded) => {
        totalLoaded += loaded;
    });

    const percentComplete = totalBytes > 0 ? Math.round((totalLoaded / totalBytes) * 100) : 0;

    // Update UI
    const globalProgressBar = document.getElementById('global-progress-bar');
    const globalProgressLabel = document.getElementById('global-progress-label');
    const globalProgressText = document.getElementById('global-progress-text');

    if (globalProgressBar) {
        globalProgressBar.style.width = `${percentComplete}%`;
    }

    if (globalProgressLabel) {
        if (activeFiles.size > 0) {
            // Join all active filenames, perhaps truncated if too long? For now, just join them.
            const filenames = Array.from(activeFiles).join(', ');
            globalProgressLabel.textContent = `Uploading: ${filenames}`;
        } else if (percentComplete >= 100) {
            globalProgressLabel.textContent = "Processing...";
        } else {
            globalProgressLabel.textContent = "Waiting...";
        }
    }

    if (globalProgressText) {
        globalProgressText.textContent = `${percentComplete}%`;
    }
}

function uploadFile(file, onProgress, onCompleteCallback) {
    return new Promise((resolve, reject) => {
        const formData = new FormData();
        formData.append('photo', file);

        const xhr = new XMLHttpRequest();

        xhr.upload.addEventListener('progress', (event) => {
            if (event.lengthComputable) {
                onProgress(event.loaded);
            }
        });

        xhr.addEventListener('load', () => {
            // Ensure we mark this file as fully loaded in our map
            onProgress(file.size);

            if (xhr.status >= 200 && xhr.status < 300) {
                try {
                    const result = JSON.parse(xhr.responseText);
                    if (result.status === 'success') {
                        uploadedFilenames.push(result.filename);

                        // Append thumbnail to grid
                        if (result.thumbnail_url) {
                            const grid = document.getElementById('thumbnail-grid');
                            if (grid) {
                                const imgContainer = document.createElement('div');
                                imgContainer.className = 'thumbnail-item';

                                const img = document.createElement('img');
                                img.src = result.thumbnail_url;
                                img.alt = result.filename;

                                imgContainer.appendChild(img);
                                grid.appendChild(imgContainer);
                            }
                        }
                    }
                } catch (e) {
                    console.error("Error parsing response", e);
                }
            }
            if (onCompleteCallback) onCompleteCallback();
            resolve();
        });

        xhr.addEventListener('error', () => {
            // Even on error, we might want to count it as "processed" for progress bar purposes 
            // or handle it differently. For simplicity, we treat it as done.
            onProgress(file.size);
            if (onCompleteCallback) onCompleteCallback();
            resolve();
        });

        xhr.open('POST', '/upload', true);
        xhr.send(formData);
    });
}

galleryButton.addEventListener('click', () => {
    window.location.href = '/gallery';
});

uploadForm.addEventListener('submit', async (e) => {
    e.preventDefault();

    const files = fileInput.files;
    if (files.length === 0) {
        const errorMsg = document.createElement('p');
        errorMsg.style.color = 'red';
        errorMsg.textContent = 'Please select files to upload.';
        uploadStatus.innerHTML = '';
        uploadStatus.appendChild(errorMsg);
        return;
    }

    // Reset State
    uploadedFilenames.length = 0;
    uploadButton.disabled = true;
    galleryButton.style.display = 'none';
    loadedBytesMap.clear();
    activeFiles.clear();
    totalBytes = 0;

    // Calculate total size
    for (const file of files) {
        totalBytes += file.size;
        loadedBytesMap.set(file, 0);
    }

    // Initialize UI
    uploadStatus.innerHTML = `
        <div class="global-progress-container">
            <div class="global-progress-info">
                <span id="global-progress-label">Preparing...</span>
                <span id="global-progress-text">0%</span>
            </div>
            <div class="progress-bar-container large">
                <div id="global-progress-bar" class="progress-bar"></div>
            </div>
        </div>
        <div id="thumbnail-grid" class="thumbnail-grid"></div>
    `;

    const totalStartTime = performance.now();
    let completedCount = 0;
    const updateCount = () => {
        // Optional: Update a counter if we want, but user asked for "name of file currently being reflected"
        // which updateGlobalProgress handles.
    };

    // Concurrent upload logic
    const concurrencyLimit = 2;
    const queue = [...files];

    async function worker() {
        while (queue.length > 0) {
            const file = queue.shift();
            // Start of upload for this file
            activeFiles.add(file.name);
            updateGlobalProgress();

            await uploadFile(file, (loaded) => {
                loadedBytesMap.set(file, loaded);
                updateGlobalProgress();
            }, () => {
                completedCount++;
            });
            // End of upload for this file
            activeFiles.delete(file.name);
            updateGlobalProgress();
        }
    }

    const workers = [];
    for (let i = 0; i < concurrencyLimit; i++) {
        workers.push(worker());
    }

    await Promise.all(workers);

    // Finalize
    updateGlobalProgress(); // Should be cleared now

    // Show final status
    const totalEndTime = performance.now();
    const totalDuration = ((totalEndTime - totalStartTime) / 1000).toFixed(2);

    const finalMessage = document.createElement('div');
    finalMessage.className = 'upload-complete-message';
    finalMessage.innerHTML = `
        <h3>Upload Complete!</h3>
        <p>Total time: ${totalDuration}s</p>
    `;
    uploadStatus.appendChild(finalMessage);

    // Force 100% just in case
    const bar = document.getElementById('global-progress-bar');
    if (bar) bar.style.width = '100%';
    const text = document.getElementById('global-progress-text');
    if (text) text.textContent = '100%';
    const label = document.getElementById('global-progress-label');
    if (label) label.textContent = 'Done';


    uploadButton.disabled = false;
    if (uploadedFilenames.length > 0) {
        galleryButton.style.display = 'inline-block';
    }
    fileInput.value = '';
});