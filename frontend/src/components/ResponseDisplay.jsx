import { useState } from 'react'

function ResponseDisplay({ response, isLoading }) {
  const [activeTab, setActiveTab] = useState('body')

  const getStatusStyle = (status) => {
    const code = parseInt(status)
    if (code >= 200 && code < 300) return { color: 'var(--ok)', background: 'var(--ok-soft)' }
    if (code >= 300 && code < 400) return { color: 'var(--stone-700)', background: 'var(--stone-150)' }
    if (code >= 400 && code < 500) return { color: 'var(--warn)', background: 'var(--warn-soft)' }
    if (code >= 500) return { color: 'var(--bad)', background: 'var(--bad-soft)' }
    return { color: 'var(--stone-700)', background: 'var(--stone-150)' }
  }

  const formatBody = (body) => {
    if (!body) return ''
    try {
      const parsed = JSON.parse(body)
      return JSON.stringify(parsed, null, 2)
    } catch {
      return body
    }
  }

  const getResponseSize = (body) => {
    if (!body) return '0 B'
    const bytes = new Blob([body]).size
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  }

  if (isLoading) {
    return (
      <div className="response-display" aria-busy="true">
        <div className="response-header">
          <h3>Response</h3>
          <div className="response-info" aria-hidden="true">
            <span className="skeleton-pill skeleton-pill-status" />
            <span className="skeleton-pill skeleton-pill-meta" />
            <span className="skeleton-pill skeleton-pill-meta" />
          </div>
        </div>
        <div className="response-tabs" aria-hidden="true">
          <span className="tab-button active">Body</span>
          <span className="tab-button">Headers</span>
        </div>
        <div className="response-content">
          <div className="response-body">
            <div className="response-skeleton" aria-hidden="true">
              <div className="skeleton-line" />
              <div className="skeleton-line skeleton-line-90" />
              <div className="skeleton-line skeleton-line-70" />
              <div className="skeleton-line" />
              <div className="skeleton-line skeleton-line-50" />
            </div>
            <span className="visually-hidden">Sending request…</span>
          </div>
        </div>
      </div>
    )
  }

  if (!response) {
    return (
      <div className="response-display">
        <div className="response-header">
          <h3>Response</h3>
        </div>
        <div className="empty-response">
          <p>No response yet.</p>
        </div>
      </div>
    )
  }

  if (response.error) {
    return (
      <div className="response-display">
        <div className="response-header">
          <h3>Response</h3>
          <div className="response-info">
            <span className="status-badge error">Error</span>
            {response.time && <span className="response-time">{response.time}</span>}
          </div>
        </div>
        <div className="error-response">
          <p className="error-message">{response.message}</p>
        </div>
      </div>
    )
  }

  const headerCount = Object.keys(response.headers || {}).length

  return (
    <div className="response-display">
      <div className="response-header">
        <h3>Response</h3>
        <div className="response-info">
          <span
            className="status-badge"
            style={getStatusStyle(response.status)}
          >
            {response.status}
          </span>
          <span className="response-time">
            {response.time || '0ms'}
          </span>
          <span className="response-time">
            {getResponseSize(response.body)}
          </span>
        </div>
      </div>

      <div className="response-tabs">
        <button
          className={activeTab === 'body' ? 'tab-button active' : 'tab-button'}
          onClick={() => setActiveTab('body')}
        >
          Body
        </button>
        
        <button
          className={activeTab === 'headers' ? 'tab-button active' : 'tab-button'}
          onClick={() => setActiveTab('headers')}
        >
          Headers{headerCount > 0 ? ` (${headerCount})` : ''}
        </button>
      </div>

      <div className="response-content">
        {activeTab === 'body' && (
          <div className="response-body">
            <pre className="response-body-content">
              {response.body ? formatBody(response.body) : 'No response body'}
            </pre>
          </div>
        )}

        {activeTab === 'headers' && (
          <div className="response-headers">
            {headerCount > 0 ? (
              <div className="headers-list">
                {Object.entries(response.headers)
                  .sort(([a], [b]) => a.localeCompare(b))
                  .map(([key, value]) => (
                    <div key={key} className="header-item">
                      <span className="header-key">{key}:</span>
                      <span className="header-value">
                        {Array.isArray(value) ? value.join(', ') : value}
                      </span>
                    </div>
                  ))}
              </div>
            ) : (
              <p className="no-headers">No response headers</p>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

export default ResponseDisplay
