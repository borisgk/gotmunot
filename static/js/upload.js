const uploadForm = document.getElementById('uploadForm');
const fileInput = document.getElementById('photo');
const uploadButton = document.getElementById('uploadButton');
const uploadStatus = document.getElementById('upload-status');
const galleryButton = document.getElementById('galleryButton');
const uploadedFilenames = []; // To store filenames for batch processing

function uploadFile(file, itemContainer, onCompleteCallback) {
    return new Promise((resolve, reject) => {
        const formData = new FormData();
        formData.append('photo', file);

        // Get the elements from the pre-populated container
        const itemLabel = itemContainer.querySelector('.upload-item-label');
        const progressBar = itemContainer.querySelector('.progress-bar');

        itemLabel.textContent = `Uploading ${file.name}...`;
        // Scroll this item into view when its upload starts.
        itemContainer.scrollIntoView({ behavior: 'smooth', block: 'nearest' });

        const startTime = performance.now(); // Record start time

        // Use XMLHttpRequest for progress events (fetch doesn't support upload progress yet)
        const xhr = new XMLHttpRequest();

        xhr.upload.addEventListener('progress', (event) => {
            if (event.lengthComputable) {
                const percentComplete = Math.round((event.loaded / event.total) * 100);
                progressBar.style.width = `${percentComplete}%`;
            }
        });

        xhr.addEventListener('load', () => {
            const endTime = performance.now(); // Record end time
            const duration = ((endTime - startTime) / 1000).toFixed(2); // Calculate duration in seconds
            if (xhr.status >= 200 && xhr.status < 300) {
                try {
                    const result = JSON.parse(xhr.responseText);
                    if (result.status === 'success') {
                        itemLabel.textContent = `✅ ${file.name} - Complete`;
                        progressBar.style.backgroundColor = '#28a745'; // Success green
                        uploadedFilenames.push(result.filename); // Collect filename
                    } else {
                        itemLabel.textContent = `❌ ${file.name} - Error: ${result.message}`;
                        progressBar.style.backgroundColor = '#dc3545'; // Error red
                    }
                } catch (e) {
                    itemLabel.textContent = `❌ ${file.name} - Error parsing server response.`;
                    progressBar.style.backgroundColor = '#dc3545';
                }
            } else {
                itemLabel.textContent = `❌ ${file.name} - Server error: ${xhr.statusText}`;
                progressBar.style.backgroundColor = '#dc3545';
            }
            if (onCompleteCallback) onCompleteCallback();
            resolve();
        });

        xhr.addEventListener('error', () => {
            itemLabel.textContent = `❌ ${file.name} - Network or server error.`;
            progressBar.style.backgroundColor = '#dc3545';
            if (onCompleteCallback) onCompleteCallback();
            resolve(); // Resolve so the next file can start
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
        if (files.length === 0) {
            const errorMsg = document.createElement('p');
            errorMsg.style.color = 'red';
            errorMsg.textContent = 'Please select files to upload.';
            uploadStatus.innerHTML = '';
            uploadStatus.appendChild(errorMsg);
            return;
        }
    }
    uploadedFilenames.length = 0; // Clear previous uploads
    uploadButton.disabled = true;
    galleryButton.style.display = 'none';

    const totalFiles = files.length;
    let completedCount = 0;
    const progressHeading = document.createElement('h3');
    const updateHeading = () => {
        progressHeading.textContent = `Upload Progress (${completedCount}/${totalFiles})`;
    };
    updateHeading(); // Initial text
    uploadStatus.innerHTML = ''; // Clear previous content
    uploadStatus.appendChild(progressHeading);

    const totalStartTime = performance.now();

    // Create a map to hold file-to-element references
    const fileElementMap = new Map();

    // Pre-populate the UI with all filenames as placeholders
    for (const file of files) {
        const itemDiv = document.createElement('div');
        itemDiv.className = 'upload-item';

        const itemInner = document.createElement('div');
        itemInner.className = 'upload-item-inner';

        const thumb = document.createElement('img');
        thumb.className = 'upload-item-thumb';

        // Generate client-side thumbnail
        const reader = new FileReader();
        reader.onload = (e) => { thumb.src = e.target.result; };
        reader.readAsDataURL(file);

        const progressWrapper = document.createElement('div');
        progressWrapper.style.flexGrow = '1';

        const itemLabel = document.createElement('div');
        itemLabel.className = 'upload-item-label';
        itemLabel.textContent = file.name;

        const progressContainer = document.createElement('div');
        progressContainer.className = 'progress-bar-container';
        const progressBar = document.createElement('div');
        progressBar.className = 'progress-bar';
        progressContainer.appendChild(progressBar);

        progressWrapper.append(itemLabel, progressContainer);
        itemInner.append(thumb, progressWrapper);
        itemDiv.appendChild(itemInner);
        uploadStatus.appendChild(itemDiv);
        fileElementMap.set(file, itemDiv);
    }

    // --- Concurrent upload with a limit ---
    const concurrencyLimit = 2;
    const queue = [...files]; // Create a mutable copy of the files array

    async function worker() {
        while (queue.length > 0) {
            const file = queue.shift(); // Get the next file from the queue
            await uploadFile(file, fileElementMap.get(file), () => {
                completedCount++;
                updateHeading();
            });
        }
    }

    // Create and start the workers
    const workers = [];
    for (let i = 0; i < concurrencyLimit; i++) {
        workers.push(worker());
    }

    await Promise.all(workers); // Wait for all workers to finish

    const totalEndTime = performance.now();
    const totalDuration = ((totalEndTime - totalStartTime) / 1000).toFixed(2);
    const totalTimeElement = document.createElement('p');
    totalTimeElement.innerHTML = `<strong>Total upload time: ${totalDuration} seconds.</strong>`;
    uploadStatus.appendChild(totalTimeElement);

    uploadButton.disabled = false;
    if (uploadedFilenames.length > 0) {
        galleryButton.style.display = 'inline-block';
    }
    fileInput.value = ''; // Clear the file input more reliably
});