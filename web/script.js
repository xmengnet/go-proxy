document.addEventListener('DOMContentLoaded', () => {
    fetch('/api/stats')
        .then(response => response.json())
        .then(data => {
            const proxyList = document.getElementById('proxy-list');
            proxyList.innerHTML = ''; // Clear existing content

            if (data && data.length > 0) {
                data.forEach(proxy => {
                    const proxyItem = document.createElement('div');
                    proxyItem.classList.add('proxy-item');

                    // Construct the full proxy URL
                    const fullProxyUrl = `${window.location.origin}${proxy.proxy_url}`;

                    const proxyInfoDiv = document.createElement('div');
                    proxyInfoDiv.classList.add('proxy-info');

                    const proxyUrlDiv = document.createElement('div');
                    proxyUrlDiv.classList.add('proxy-url');
                    proxyUrlDiv.innerHTML = `<i class="fas fa-link icon"></i><strong>Proxy:</strong> <a href="${fullProxyUrl}" target="_blank">${fullProxyUrl}</a>`;
                    proxyInfoDiv.appendChild(proxyUrlDiv);

                    const sourceUrlDiv = document.createElement('div');
                    sourceUrlDiv.classList.add('source-url');
                    sourceUrlDiv.innerHTML = `<i class="fas fa-globe icon"></i><strong>Source:</strong> ${proxy.source_url}`;
                    proxyInfoDiv.appendChild(sourceUrlDiv);

                    const accessCountDiv = document.createElement('div');
                    accessCountDiv.classList.add('access-count');
                    accessCountDiv.innerHTML = `<i class="fas fa-chart-bar icon"></i><strong>Requests:</strong> ${proxy.access_count}`;
                    proxyInfoDiv.appendChild(accessCountDiv);

                    proxyItem.appendChild(proxyInfoDiv);

                    const copyButton = document.createElement('button');
                    copyButton.classList.add('copy-button');
                    copyButton.textContent = 'Copy';
                    copyButton.onclick = () => {
                        navigator.clipboard.writeText(fullProxyUrl).then(() => {
                            copyButton.textContent = 'Copied!';
                            setTimeout(() => {
                                copyButton.textContent = 'Copy';
                            }, 2000);
                        }).catch(err => {
                            console.error('Failed to copy: ', err);
                        });
                    };
                    proxyItem.appendChild(copyButton);

                    proxyList.appendChild(proxyItem);
                });
            } else {
                proxyList.innerHTML = '<p>No proxy statistics available.</p>';
            }
        })
        .catch(error => {
            console.error('Error fetching proxy statistics:', error);
            document.getElementById('proxy-list').innerHTML = '<p>Error loading statistics.</p>';
        });
});
