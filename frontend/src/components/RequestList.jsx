import { useState, useEffect } from 'react'

function RequestList({ requests, onRequestSelect, selectedRequest, onImportOpenAPI, onPreviewOpenAPI, onExportCollection, onClearImportStatus, importStatus }) {
  const [expandedFolders, setExpandedFolders] = useState(new Set())
  const [searchTerm, setSearchTerm] = useState('')
  const [pendingFile, setPendingFile] = useState(null)
  const [pendingCollection, setPendingCollection] = useState('')
  const [pendingPreview, setPendingPreview] = useState(null)
  const [previewError, setPreviewError] = useState('')
  const importInputId = 'openapi-import-input'
  const loadingLabel = importStatus?.message?.startsWith('Exporting') ? 'Exporting...' : 'Importing...'

  useEffect(() => {
    setExpandedFolders(new Set(Object.keys(requests)))
  }, [requests])

  useEffect(() => {
    if (importStatus?.type === 'success') {
      setPendingFile(null)
      setPendingCollection('')
      setPendingPreview(null)
      setPreviewError('')
    }
  }, [importStatus])

  const handleFilePicked = async (file) => {
    if (!file) return
    setPendingFile(file)
    setPendingCollection('')
    setPendingPreview(null)
    setPreviewError('')
    onClearImportStatus?.()
    try {
      const preview = await onPreviewOpenAPI?.(file)
      if (preview) {
        setPendingPreview(preview)
        setPendingCollection(preview.suggestedCollection || '')
      }
    } catch (err) {
      setPreviewError(err.message || 'Could not parse spec.')
    }
  }

  const handleCancelImport = () => {
    setPendingFile(null)
    setPendingCollection('')
    setPendingPreview(null)
    setPreviewError('')
    onClearImportStatus?.()
  }

  const handleConfirmImport = (overwrite = false) => {
    if (!pendingFile) return
    onImportOpenAPI({
      file: pendingFile,
      collection: pendingCollection,
      overwrite,
    })
  }

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
            {importStatus?.type === 'loading' ? loadingLabel : 'Import'}
          </label>
          <input
            id={importInputId}
            type="file"
            accept=".json,.yaml,.yml,application/json,application/yaml,text/yaml,text/x-yaml"
            className="import-file-input"
            disabled={importStatus?.type === 'loading'}
            onChange={(e) => {
              const file = e.target.files?.[0]
              handleFilePicked(file)
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

        {pendingFile && importStatus?.type !== 'conflict' && (
          <div className="import-form">
            <div className="import-form-row">
              <span className="import-form-label">File</span>
              <span className="import-form-value">{pendingFile.name}</span>
            </div>
            <label className="import-form-row import-form-name">
              <span className="import-form-label">Collection</span>
              <input
                type="text"
                className="import-form-input"
                value={pendingCollection}
                onChange={(e) => setPendingCollection(e.target.value)}
                placeholder={pendingPreview?.suggestedCollection || 'collection-name'}
                spellCheck={false}
                autoFocus
                disabled={importStatus?.type === 'loading'}
              />
            </label>
            <div className="import-form-path">
              requests/{pendingCollection || pendingPreview?.suggestedCollection || 'api'}/
            </div>

            {previewError && (
              <div className="import-form-banner error">{previewError}</div>
            )}
            {pendingPreview && (() => {
              const name = pendingCollection.trim() || pendingPreview.suggestedCollection
              const matchesSuggestion = name === pendingPreview.suggestedCollection
              const isCollectionBundle = pendingPreview.type === 'collection'
              const itemCount = isCollectionBundle ? pendingPreview.requests : pendingPreview.operations
              const itemLabel = isCollectionBundle ? 'request' : 'operation'
              const opLabel = `${itemCount} ${itemLabel}${itemCount === 1 ? '' : 's'}`
              const bodyLabel = isCollectionBundle && pendingPreview.bodies
                ? ` Includes ${pendingPreview.bodies} persisted bod${pendingPreview.bodies === 1 ? 'y' : 'ies'}.`
                : ''

              if (matchesSuggestion && pendingPreview.exists && pendingPreview.ownedBySpec) {
                return (
                  <div className="import-form-banner update">
                    {isCollectionBundle
                      ? `Replacing existing collection. ${opLabel}.${bodyLabel}`
                      : `Overwriting spec-owned collection. ${opLabel}. Generated requests will be rewritten; operations no longer in this spec will be pruned.`}
                  </div>
                )
              }
              if (matchesSuggestion && pendingPreview.exists && !pendingPreview.ownedBySpec) {
                return (
                  <div className="import-form-banner warn">
                    Folder exists but isn't spec-owned. Rename, or confirm overwrite on submit.
                  </div>
                )
              }
              if (!matchesSuggestion) {
                return (
                  <div className="import-form-banner info">
                    New folder. {opLabel}.{bodyLabel} Server will confirm if the renamed target collides.
                  </div>
                )
              }
              return (
                <div className="import-form-banner info">
                  New collection. {opLabel}.{bodyLabel}
                </div>
              )
            })()}

            {(() => {
              const name = pendingCollection.trim() || pendingPreview?.suggestedCollection
              const matchesSuggestion = pendingPreview && name === pendingPreview.suggestedCollection
              const isUpdate = matchesSuggestion && pendingPreview.exists && pendingPreview.ownedBySpec
              const confirmClass = isUpdate
                ? 'import-form-confirm destructive-mild'
                : 'import-form-confirm'
              const confirmLabel = importStatus?.type === 'loading'
                ? 'Importing...'
                : isUpdate
                ? 'Update'
                : 'Import'
              const confirmTitle = isUpdate
                ? pendingPreview.type === 'collection'
                  ? 'Replaces the existing collection with the exported bundle'
                  : 'Rewrites generated requests and prunes operations no longer in this spec'
                : undefined
              const submitOverwrite = isUpdate && pendingPreview.type === 'collection'
              return (
                <div className="import-form-actions">
                  <button
                    className="import-form-cancel"
                    onClick={handleCancelImport}
                    disabled={importStatus?.type === 'loading'}
                    type="button"
                  >
                    Cancel
                  </button>
                  <button
                    className={confirmClass}
                    onClick={() => handleConfirmImport(submitOverwrite)}
                    disabled={!pendingCollection.trim() || importStatus?.type === 'loading' || !!previewError}
                    type="button"
                    title={confirmTitle}
                  >
                    {confirmLabel}
                  </button>
                </div>
              )
            })()}
          </div>
        )}

        {importStatus?.type === 'conflict' && (
          <div className="import-status conflict">
            <div className="import-status-message">{importStatus.message}</div>
            <div className="import-form-row import-form-name">
              <span className="import-form-label">Rename</span>
              <input
                type="text"
                className="import-form-input"
                value={pendingCollection}
                onChange={(e) => setPendingCollection(e.target.value)}
                spellCheck={false}
                autoFocus
              />
            </div>
            <div className="import-form-path">
              requests/{pendingCollection || 'api'}/
            </div>
            <div className="import-form-actions">
              <button
                className="import-form-cancel"
                onClick={handleCancelImport}
                type="button"
              >
                Cancel
              </button>
              <button
                className="import-form-confirm destructive"
                onClick={() => handleConfirmImport(true)}
                type="button"
                title="Wipes the existing folder, including hand-rolled requests that are not in this spec"
              >
                Overwrite
              </button>
              <button
                className="import-form-confirm"
                onClick={() => handleConfirmImport(false)}
                disabled={!pendingCollection.trim() || pendingCollection === importStatus.collection}
                type="button"
                title="Import using the new collection name"
              >
                Import as new
              </button>
            </div>
          </div>
        )}

        {importStatus?.message && importStatus.type !== 'conflict' && (
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
                <button
                  type="button"
                  className="folder-export"
                  onClick={(e) => {
                    e.stopPropagation()
                    onExportCollection?.(folder)
                  }}
                  title={`Export ${folder} with saved bodies`}
                >
                  Export
                </button>
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
