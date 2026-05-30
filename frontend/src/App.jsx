import { useState, useEffect, useCallback } from 'react'
import './App.css'
import RequestBuilder from './components/RequestBuilder'
import RequestList from './components/RequestList'
import ResponseDisplay from './components/ResponseDisplay'
import EnvironmentSelector from './components/EnvironmentSelector'
import CollectionEnvironmentEditor from './components/CollectionEnvironmentEditor'

function App() {
  const [selectedRequest, setSelectedRequest] = useState(null)
  const [response, setResponse] = useState(null)
  const [isLoading, setIsLoading] = useState(false)
  const [environments, setEnvironments] = useState([])
  const [selectedEnv, setSelectedEnv] = useState('dev')
  const [requests, setRequests] = useState({})
  const [activeCollection, setActiveCollection] = useState(null)
  const [collectionEnvs, setCollectionEnvs] = useState(null)
  const [showEnvEditor, setShowEnvEditor] = useState(false)
  const [lastCurlRequest, setLastCurlRequest] = useState('')
  const [lastExecutedRequest, setLastExecutedRequest] = useState(null)
  const [importStatus, setImportStatus] = useState(null)

  useEffect(() => {
    loadInitialData()
  }, [])

  const loadCollectionEnvs = useCallback(async (collection) => {
    if (!collection) {
      setCollectionEnvs(null)
      return
    }
    try {
      const res = await fetch(`/api/collection-environments/${encodeURIComponent(collection)}`)
      if (res.ok) {
        const data = await res.json()
        setCollectionEnvs(data)
      }
    } catch (err) {
      console.error('Error loading collection environments:', err)
    }
  }, [])

  const loadInitialData = async () => {
    try {
      const envResponse = await fetch('/api/environments')
      if (envResponse.ok) {
        const envData = await envResponse.json()
        setEnvironments(envData)
      }

      const reqResponse = await fetch('/api/requests')
      if (reqResponse.ok) {
        const reqData = await reqResponse.json()
        setRequests(reqData)
      }
    } catch (error) {
      console.error('Error loading initial data:', error)
    }
  }

  const handleCreateEnvironment = async (name, source) => {
    const res = await fetch('/api/environments', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, source: source || undefined }),
    })
    if (!res.ok) {
      let message = `Failed to create environment (${res.status})`
      try {
        const data = await res.json()
        if (data?.error) message = data.error
      } catch { /* ignore parse error, keep status fallback */ }
      throw new Error(message)
    }
    const updated = await res.json()
    setEnvironments(updated)
    return updated
  }

  const handleRequestSelect = (request) => {
    setSelectedRequest(request)
    setResponse(null)
    setLastCurlRequest('')
    setLastExecutedRequest(null)

    const collection = request?.path?.split('/')[0] || null
    if (collection !== activeCollection) {
      setActiveCollection(collection)
      loadCollectionEnvs(collection)
    }
  }

  const previewOpenAPI = async (file, collection = '') => {
    const formData = new FormData()
    formData.append('spec', file)
    if (collection) formData.append('collection', collection)
    const res = await fetch('/api/openapi-preview', { method: 'POST', body: formData })
    if (!res.ok) {
      const message = await res.text()
      throw new Error(message.trim() || 'Preview failed')
    }
    return await res.json()
  }

  const handleImportOpenAPI = async ({ file, collection = '', overwrite = false }) => {
    if (!file) return

    setImportStatus({ type: 'loading', message: `Importing ${file.name}...` })
    const formData = new FormData()
    formData.append('spec', file)
    if (collection) formData.append('collection', collection)
    if (overwrite) formData.append('overwrite', 'true')

    try {
      const res = await fetch('/api/import-openapi', {
        method: 'POST',
        body: formData,
      })

      if (res.status === 409) {
        const conflict = await res.json().catch(() => ({}))
        setImportStatus({
          type: 'conflict',
          file,
          collection: conflict.suggestedCollection || collection,
          message: conflict.message || 'Collection already exists.',
        })
        return
      }

      if (!res.ok) {
        const message = await res.text()
        throw new Error(message.trim() || 'Import failed')
      }

      const result = await res.json()
      const reqResponse = await fetch('/api/requests')
      if (reqResponse.ok) {
        const reqData = await reqResponse.json()
        setRequests(reqData)
      }

      setActiveCollection(result.collection)
      loadCollectionEnvs(result.collection)
      const parts = [`Imported ${result.imported} requests into ${result.collection}`]
      if (result.bodies) parts.push(`${result.bodies} bodies`)
      if (result.pruned) parts.push(`pruned ${result.pruned} stale`)
      if (result.specPath) parts.push(`spec saved to ${result.specPath}`)
      setImportStatus({
        type: 'success',
        message: parts.join('. ') + '.',
      })
    } catch (error) {
      setImportStatus({ type: 'error', message: error.message })
    }
  }

  const handleClearImportStatus = () => setImportStatus(null)

  const handleExportCollection = async (collection) => {
    if (!collection) return
    setImportStatus({ type: 'loading', message: `Exporting ${collection}...` })

    try {
      const res = await fetch(`/api/export-collection/${encodeURIComponent(collection)}`)
      if (!res.ok) {
        const message = await res.text()
        throw new Error(message.trim() || 'Export failed')
      }

      const blob = await res.blob()
      const url = window.URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `${collection}.api-man-collection.json`
      document.body.appendChild(link)
      link.click()
      link.remove()
      window.URL.revokeObjectURL(url)
      setImportStatus({
        type: 'success',
        message: `Exported ${collection}. Import this file to restore requests and saved bodies.`,
      })
    } catch (error) {
      setImportStatus({ type: 'error', message: error.message })
    }
  }

  const handleSaveCollectionEnvs = async (updatedEnvs) => {
    if (!activeCollection) return
    try {
      const res = await fetch(`/api/collection-environments/${encodeURIComponent(activeCollection)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(updatedEnvs),
      })
      if (res.ok) {
        setCollectionEnvs(updatedEnvs)
      }
    } catch (err) {
      console.error('Error saving collection environments:', err)
    }
  }

  const handleExecuteRequest = async (requestData) => {
    setIsLoading(true)
    setLastCurlRequest('')
    setLastExecutedRequest(null)
    try {
      const res = await fetch('/api/execute', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          request: requestData,
          environment: selectedEnv,
          collection: activeCollection || '',
        }),
      })

      const result = await res.json()
      setResponse(result)
      setLastCurlRequest(result.curl || '')
      setLastExecutedRequest(result.request || null)
    } catch (error) {
      setResponse({ error: true, message: error.message })
      setLastCurlRequest('')
      setLastExecutedRequest(null)
    } finally {
      setIsLoading(false)
    }
  }

  const hasCollectionEnv = collectionEnvs?.environments?.[selectedEnv]
  const activeEnvSource = hasCollectionEnv ? 'collection' : 'global'
  const activeEnvDisplay = hasCollectionEnv
    ? collectionEnvs.environments[selectedEnv]
    : environments.find(e => e.name === selectedEnv)

  return (
    <div className="app">
      <header className="app-header">
        <div className="header-left">
          <h1>API-Man</h1>
          {activeCollection && (
            <span className="active-collection">{activeCollection}</span>
          )}
        </div>
        <div className="header-right">
          <EnvironmentSelector
            environments={environments}
            selected={selectedEnv}
            onChange={setSelectedEnv}
            envSource={activeEnvSource}
            activeBaseURL={
              hasCollectionEnv
                ? collectionEnvs.environments[selectedEnv].baseURL
                : activeEnvDisplay?.baseURL
            }
            onCreateEnvironment={handleCreateEnvironment}
          />
          {activeCollection && (
            <button
              className="env-edit-button"
              onClick={() => setShowEnvEditor(!showEnvEditor)}
              title="Edit collection environments"
            >
              {showEnvEditor ? 'Close' : 'Env Config'}
            </button>
          )}
        </div>
      </header>

      <div className="app-body">
        <div className="left-panel">
          <RequestList
            requests={requests}
            onRequestSelect={handleRequestSelect}
            selectedRequest={selectedRequest}
            onImportOpenAPI={handleImportOpenAPI}
            onPreviewOpenAPI={previewOpenAPI}
            onExportCollection={handleExportCollection}
            onClearImportStatus={handleClearImportStatus}
            importStatus={importStatus}
          />
        </div>

        <div className="main-panel">
          {showEnvEditor && activeCollection ? (
            <CollectionEnvironmentEditor
              collection={activeCollection}
              collectionEnvs={collectionEnvs}
              globalEnvironments={environments}
              selectedEnv={selectedEnv}
              onSave={handleSaveCollectionEnvs}
              onClose={() => setShowEnvEditor(false)}
            />
          ) : (
            <>
              <RequestBuilder
                request={selectedRequest}
                onExecute={handleExecuteRequest}
                isLoading={isLoading}
                curlRequest={lastCurlRequest}
                executedRequest={lastExecutedRequest}
              />
              <ResponseDisplay
                response={response}
                isLoading={isLoading}
              />
            </>
          )}
        </div>
      </div>
    </div>
  )
}

export default App
