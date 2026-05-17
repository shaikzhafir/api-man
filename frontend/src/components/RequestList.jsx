import { useState, useEffect } from 'react'

function RequestList({ requests, onRequestSelect, selectedRequest, onImportOpenAPI, importStatus }) {
  const [expandedFolders, setExpandedFolders] = useState(new Set())
  const [searchTerm, setSearchTerm] = useState('')
  const importInputId = 'openapi-import-input'

  useEffect(() => {
    setExpandedFolders(new Set(Object.keys(requests)))
  }, [requests])

  const toggleFolder = (folder) => {
    const newExpanded = new Set(expandedFolders)
    if (newExpanded.has(folder)) {
      newExpanded.delete(folder)
    } else {
      newExpanded.add(folder)
    }
    setExpandedFolders(newExpanded)
  }

  const getMethodColor = (method) => {
    if (method?.toUpperCase() === 'DELETE') return 'var(--bad)'
    return undefined
  }

  const isRequestPath = (path, allPaths) => {
    if (path.endsWith('/request')) return true
    const hasSubRequest = allPaths.some(p => p === `${path}/request` || p.startsWith(`${path}/`))
    return !hasSubRequest
  }

  const formatRequestName = (name) => {
    return name
      .replace(/^(get|post|put|delete|patch|head|options)/i, '')
      .replace(/^-/, '')
      .replace(/-/g, ' ')
      .replace(/\{([^}]+)\}/g, ':$1')
      .replace(/([a-z])([A-Z])/g, '$1 $2')
      .trim() || name
  }

  const getMethodFromRequestName = (name) => {
    const method = ['OPTIONS', 'DELETE', 'PATCH', 'POST', 'HEAD', 'PUT', 'GET']
      .find(candidate => name.toUpperCase().startsWith(candidate))
    return method || 'GET'
  }

  const getRequestEntry = (requestPath) => {
    const actualPath = requestPath.endsWith('/request')
      ? requestPath.replace(/\/request$/, '')
      : requestPath
    const pathParts = actualPath.split('/')
    const requestName = pathParts[pathParts.length - 1]
    const displayMethod = getMethodFromRequestName(requestName)
    const displayName = formatRequestName(requestName)

    return {
      actualPath,
      requestName,
      displayName,
      displayMethod,
    }
  }

  const normalizedSearch = searchTerm.trim().toLowerCase()

  return (
    <div className="request-list">
      <div className="request-list-header">
        <div className="request-list-title-row">
          <h3>Requests</h3>
          <label
            className={`import-button ${importStatus?.type === 'loading' ? 'loading' : ''}`}
            htmlFor={importInputId}
          >
            {importStatus?.type === 'loading' ? 'Importing...' : 'Import'}
          </label>
          <input
            id={importInputId}
            type="file"
            accept=".json,.yaml,.yml,application/json,application/yaml,text/yaml,text/x-yaml"
            className="import-file-input"
            disabled={importStatus?.type === 'loading'}
            onChange={(e) => {
              const file = e.target.files?.[0]
              onImportOpenAPI(file)
              e.target.value = ''
            }}
          />
        </div>
        <input
          type="search"
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          className="request-search"
          placeholder="Search requests"
        />
        {importStatus?.message && (
          <div className={`import-status ${importStatus.type}`}>
            {importStatus.message}
          </div>
        )}
      </div>
      
      <div className="request-list-content">
        {Object.entries(requests).map(([folder, requestList]) => {
          const requestEntries = requestList
            .filter(path => isRequestPath(path, requestList))
            .map(getRequestEntry)
          const folderMatches = normalizedSearch && folder.toLowerCase().includes(normalizedSearch)
          const visibleRequests = normalizedSearch && !folderMatches
            ? requestEntries.filter(({ actualPath, displayName, displayMethod }) => (
                actualPath.toLowerCase().includes(normalizedSearch) ||
                displayName.toLowerCase().includes(normalizedSearch) ||
                displayMethod.toLowerCase().includes(normalizedSearch)
              ))
            : requestEntries
          const requestCount = visibleRequests.length

          if (normalizedSearch && requestCount === 0) {
            return null
          }

          return (
            <div key={folder} className="request-folder">
              <div 
                className="folder-header"
                onClick={() => toggleFolder(folder)}
              >
                <span className={`folder-icon ${expandedFolders.has(folder) || normalizedSearch ? 'expanded' : ''}`}>
                  ▶
                </span>
                <span className="folder-name">{folder}</span>
                <span className="request-count">{requestCount}</span>
              </div>
              
              {(expandedFolders.has(folder) || normalizedSearch) && (
                <div className="request-items">
                  {visibleRequests.map(({ actualPath, requestName, displayName, displayMethod }) => {
                    const isSelected = selectedRequest && 
                      selectedRequest.path === actualPath
                    
                    const handleRequestClick = async () => {
                      try {
                        const response = await fetch(`/api/request/${encodeURIComponent(actualPath)}`)
                        if (response.ok) {
                          const requestDetails = await response.json()
                          onRequestSelect({
                            path: actualPath,
                            name: requestDetails.name || requestName,
                            method: requestDetails.method || displayMethod,
                            url: requestDetails.url || `/${requestName.replace(/^(get|post|put|delete|patch)-/, '').replace(/-/g, '/')}`,
                            headers: requestDetails.headers || {},
                            params: requestDetails.params || {},
                            body: requestDetails.body || '',
                          })
                        } else {
                          onRequestSelect({ 
                            path: actualPath,
                            name: requestName,
                            method: displayMethod,
                            url: `/${requestName.replace(/^(get|post|put|delete|patch)-/, '').replace(/-/g, '/')}`,
                            headers: {},
                            params: {},
                            body: '',
                          })
                        }
                      } catch (error) {
                        console.error('Error loading request details:', error)
                        onRequestSelect({ 
                          path: actualPath,
                          name: requestName,
                          method: displayMethod,
                          url: `/${requestName.replace(/^(get|post|put|delete|patch)-/, '').replace(/-/g, '/')}`,
                          headers: {},
                          params: {},
                          body: '',
                        })
                      }
                    }
                    
                    return (
                      <div
                        key={actualPath}
                        className={`request-item ${isSelected ? 'selected' : ''}`}
                        onClick={handleRequestClick}
                      >
                        <span
                          className="method-badge"
                          style={{ color: getMethodColor(displayMethod) }}
                        >
                          {displayMethod}
                        </span>
                        <span className="request-name">
                          {displayName}
                        </span>
                      </div>
                    )
                  })}
                </div>
              )}
            </div>
          )
        })}
        
        {Object.keys(requests).length === 0 ? (
          <div className="empty-requests">
            <p>No requests found</p>
            <p className="help-text">
              Generate requests using: <code>api-man generate spec.yaml</code>
            </p>
          </div>
        ) : normalizedSearch && Object.entries(requests).every(([folder, requestList]) => {
          const requestEntries = requestList
            .filter(path => isRequestPath(path, requestList))
            .map(getRequestEntry)
          return !folder.toLowerCase().includes(normalizedSearch) &&
            requestEntries.every(({ actualPath, displayName, displayMethod }) => (
              !actualPath.toLowerCase().includes(normalizedSearch) &&
              !displayName.toLowerCase().includes(normalizedSearch) &&
              !displayMethod.toLowerCase().includes(normalizedSearch)
            ))
        }) && (
          <div className="empty-requests">
            <p>No matching requests</p>
          </div>
        )}
      </div>
    </div>
  )
}

export default RequestList
