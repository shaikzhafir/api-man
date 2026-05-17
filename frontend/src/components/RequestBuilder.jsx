import { useState, useEffect } from 'react'

const HTTP_METHODS = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'HEAD', 'OPTIONS']

function RequestBuilder({ request, onExecute, isLoading, curlRequest, executedRequest }) {
  const [method, setMethod] = useState('GET')
  const [url, setUrl] = useState('')
  const [urlTemplate, setUrlTemplate] = useState('')
  const [pathParams, setPathParams] = useState([])
  const [queryParams, setQueryParams] = useState([{ key: '', value: '' }])
  const [headers, setHeaders] = useState([{ key: '', value: '' }])
  const [body, setBody] = useState('')
  const [activeTab, setActiveTab] = useState('path')
  const [showCurl, setShowCurl] = useState(() => {
    try {
      return window.localStorage.getItem('api-man.showCurl') === 'true'
    } catch {
      return false
    }
  })
  const [copyState, setCopyState] = useState('idle')

  useEffect(() => {
    if (request) {
      setMethod(request.method || 'GET')
      const { baseUrl, queryRows } = splitUrlAndQueryParams(request.url || '')
      const requestParamRows = paramsToRows(request.params)
      const nextQueryParams = [...queryRows, ...requestParamRows]
      setUrlTemplate(baseUrl)
      const nextPathParams = createPathParamRows(baseUrl)
      setPathParams(nextPathParams)
      setActiveTab(nextPathParams.length > 0 ? 'path' : 'query')
      setUrl(buildUrlWithQueryParams(buildUrlWithPathParams(baseUrl, nextPathParams), nextQueryParams))
      setQueryParams(
        nextQueryParams.length > 0
          ? nextQueryParams
          : [{ key: '', value: '' }]
      )
      
      const headerEntries = Object.entries(request.headers || {})
      setHeaders(headerEntries.length > 0 
        ? headerEntries.map(([key, value]) => ({ key, value }))
        : [{ key: '', value: '' }]
      )
      
      let bodyStr = request.body || ''
      if (typeof bodyStr === 'object') {
        bodyStr = JSON.stringify(bodyStr, null, 2)
      }
      setBody(bodyStr)
    }
  }, [request])

  useEffect(() => {
    setCopyState('idle')
  }, [curlRequest])

  useEffect(() => {
    try {
      window.localStorage.setItem('api-man.showCurl', showCurl ? 'true' : 'false')
    } catch {
      // Ignore storage failures; the toggle still works for the current session.
    }
  }, [showCurl])

  const addHeader = () => {
    setHeaders([...headers, { key: '', value: '' }])
  }

  const removeHeader = (index) => {
    const newHeaders = headers.filter((_, i) => i !== index)
    setHeaders(newHeaders.length > 0 ? newHeaders : [{ key: '', value: '' }])
  }

  const updateHeader = (index, field, value) => {
    const newHeaders = [...headers]
    newHeaders[index] = { ...newHeaders[index], [field]: value }
    setHeaders(newHeaders)
  }

  const addQueryParam = () => {
    setQueryParams([...queryParams, { key: '', value: '' }])
  }

  const removeQueryParam = (index) => {
    const newParams = queryParams.filter((_, i) => i !== index)
    const visibleParams = newParams.length > 0 ? newParams : [{ key: '', value: '' }]
    setQueryParams(visibleParams)
    setUrl(buildUrlWithQueryParams(buildUrlWithPathParams(urlTemplate, pathParams), visibleParams))
  }

  const updateQueryParam = (index, field, value) => {
    const newParams = [...queryParams]
    newParams[index] = { ...newParams[index], [field]: value }
    setQueryParams(newParams)
    setUrl(buildUrlWithQueryParams(buildUrlWithPathParams(urlTemplate, pathParams), newParams))
  }

  const updatePathParam = (index, value) => {
    const newParams = [...pathParams]
    newParams[index] = { ...newParams[index], value }
    setPathParams(newParams)
    setUrl(buildUrlWithQueryParams(buildUrlWithPathParams(urlTemplate, newParams), queryParams))
  }

  const handleUrlChange = (value) => {
    setUrl(value)
    const { baseUrl, queryRows } = splitUrlAndQueryParams(value)
    setUrlTemplate(baseUrl)
    setPathParams(createPathParamRows(baseUrl, pathParams))
    setQueryParams(queryRows.length > 0 ? queryRows : [{ key: '', value: '' }])
  }

  const handleExecute = () => {
    const requestData = {
      method,
      url,
      headers: headers.reduce((acc, header) => {
        if (header.key && header.value) {
          acc[header.key] = header.value
        }
        return acc
      }, {}),
      body: body || undefined,
    }
    onExecute(requestData)
  }

  const copyCurlRequest = async () => {
    if (!curlRequest) return

    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(curlRequest)
      } else {
        copyTextWithFallback(curlRequest)
      }
      setCopyState('copied')
      window.setTimeout(() => setCopyState('idle'), 1500)
    } catch {
      setCopyState('failed')
    }
  }

  const copyTextWithFallback = (text) => {
    const textarea = document.createElement('textarea')
    textarea.value = text
    textarea.setAttribute('readonly', '')
    textarea.style.position = 'fixed'
    textarea.style.top = '-1000px'
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    document.body.removeChild(textarea)
  }

  const handleKeyDown = (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
      e.preventDefault()
      handleExecute()
    }
  }

  if (!request) {
    return (
      <div className="request-builder empty">
        <div className="empty-state">
          <p>No request selected.</p>
        </div>
      </div>
    )
  }

  const activeHeaderCount = headers.filter(h => h.key && h.value).length
  const activeQueryParamCount = queryParams.filter(param => param.key).length
  const activePathParamCount = pathParams.length

  return (
    <div className="request-builder" onKeyDown={handleKeyDown}>
      <div className="request-context-bar">
        <span className="request-context-name">{request.name || 'Untitled request'}</span>
        {request.path && (
          <span className="request-context-path">{request.path}</span>
        )}
      </div>

      <div className="request-line">
        <select 
          value={method} 
          onChange={(e) => setMethod(e.target.value)}
          className="method-select"
          disabled={isLoading}
        >
          {HTTP_METHODS.map(m => (
            <option key={m} value={m}>{m}</option>
          ))}
        </select>
        
        <input
          type="text"
          value={url}
          onChange={(e) => handleUrlChange(e.target.value)}
          placeholder="Enter request URL"
          className="url-input"
          disabled={isLoading}
        />
        
        <button 
          onClick={handleExecute} 
          disabled={isLoading}
          className="send-button"
        >
          {isLoading ? 'Sending...' : 'Send'}
        </button>
      </div>

      <div className="request-tabs">
        <button
          className={activeTab === 'path' ? 'tab-button active' : 'tab-button'}
          onClick={() => setActiveTab('path')}
        >
          Path{activePathParamCount > 0 ? ` (${activePathParamCount})` : ''}
        </button>

        <button
          className={activeTab === 'query' ? 'tab-button active' : 'tab-button'}
          onClick={() => setActiveTab('query')}
        >
          Query{activeQueryParamCount > 0 ? ` (${activeQueryParamCount})` : ''}
        </button>

        <button
          className={activeTab === 'headers' ? 'tab-button active' : 'tab-button'}
          onClick={() => setActiveTab('headers')}
        >
          Headers{activeHeaderCount > 0 ? ` (${activeHeaderCount})` : ''}
        </button>
        
        <button
          className={activeTab === 'body' ? 'tab-button active' : 'tab-button'}
          onClick={() => setActiveTab('body')}
        >
          Body{body ? ' *' : ''}
        </button>

        <label className={`curl-toggle ${showCurl ? 'active' : ''}`}>
          <input
            type="checkbox"
            checked={showCurl}
            onChange={(e) => setShowCurl(e.target.checked)}
          />
          Show curl after send
        </label>
      </div>

      <div className="tab-content">
        {activeTab === 'path' && (
          <div className="path-section">
            {pathParams.length > 0 ? (
              pathParams.map((param, index) => (
                <div key={`${param.key}-${index}`} className="path-param-row">
                  <div className="path-param-key">{param.key}</div>
                  <input
                    type="text"
                    placeholder={`Value for {${param.key}}`}
                    value={param.value}
                    onChange={(e) => updatePathParam(index, e.target.value)}
                    className="path-param-input"
                    disabled={isLoading}
                  />
                </div>
              ))
            ) : (
              <p className="no-path-params">No path parameters in this URL</p>
            )}
          </div>
        )}

        {activeTab === 'query' && (
          <div className="query-section">
            {queryParams.map((param, index) => (
              <div key={index} className="query-param-row">
                <input
                  type="text"
                  placeholder="Parameter name"
                  value={param.key}
                  onChange={(e) => updateQueryParam(index, 'key', e.target.value)}
                  className="query-param-input"
                  disabled={isLoading}
                />
                <input
                  type="text"
                  placeholder="Value"
                  value={param.value}
                  onChange={(e) => updateQueryParam(index, 'value', e.target.value)}
                  className="query-param-input"
                  disabled={isLoading}
                />
                <button
                  onClick={() => removeQueryParam(index)}
                  className="remove-button"
                  disabled={isLoading}
                >
                  ×
                </button>
              </div>
            ))}
            <button onClick={addQueryParam} className="add-button" disabled={isLoading}>
              + Add Param
            </button>
          </div>
        )}

        {activeTab === 'headers' && (
          <div className="headers-section">
            {headers.map((header, index) => (
              <div key={index} className="header-row">
                <input
                  type="text"
                  placeholder="Header name"
                  value={header.key}
                  onChange={(e) => updateHeader(index, 'key', e.target.value)}
                  className="header-input"
                  disabled={isLoading}
                />
                <input
                  type="text"
                  placeholder="Header value"
                  value={header.value}
                  onChange={(e) => updateHeader(index, 'value', e.target.value)}
                  className="header-input"
                  disabled={isLoading}
                />
                <button
                  onClick={() => removeHeader(index)}
                  className="remove-button"
                  disabled={isLoading}
                >
                  ×
                </button>
              </div>
            ))}
            <button onClick={addHeader} className="add-button" disabled={isLoading}>
              + Add Header
            </button>
          </div>
        )}

        {activeTab === 'body' && (
          <div className="body-section">
            <textarea
              value={body}
              onChange={(e) => setBody(e.target.value)}
              placeholder="Request body (JSON, XML, etc.)"
              className="body-textarea"
              rows="8"
              disabled={isLoading}
            />
          </div>
        )}
      </div>

      {showCurl && (
        <div className="curl-request-panel">
          <div className="curl-request-header">
            <div className="curl-request-title">
              <span>curl request</span>
              {executedRequest?.url && (
                <span className="curl-request-url">
                  {executedRequest.method} {executedRequest.url}
                </span>
              )}
            </div>
            <button
              type="button"
              className="copy-curl-button"
              onClick={copyCurlRequest}
              disabled={!curlRequest}
            >
              {copyState === 'copied' ? 'Copied' : copyState === 'failed' ? 'Copy failed' : 'Copy'}
            </button>
          </div>
          <pre className={`curl-request-content ${curlRequest ? '' : 'empty'}`}>
            {curlRequest || (isLoading ? 'Sending request...' : 'Send a request to generate a copyable curl command.')}
          </pre>
        </div>
      )}
    </div>
  )
}

function splitUrlAndQueryParams(requestUrl) {
  const queryStart = requestUrl.indexOf('?')
  if (queryStart === -1) {
    return { baseUrl: requestUrl, queryRows: [] }
  }

  const baseUrl = requestUrl.slice(0, queryStart)
  const queryAndHash = requestUrl.slice(queryStart + 1)
  const hashStart = queryAndHash.indexOf('#')
  const queryString = hashStart === -1 ? queryAndHash : queryAndHash.slice(0, hashStart)
  const params = new URLSearchParams(queryString)
  const queryRows = Array.from(params.entries()).map(([key, value]) => ({ key, value }))

  return { baseUrl, queryRows }
}

function paramsToRows(params) {
  if (!params || typeof params !== 'object') {
    return []
  }

  return Object.entries(params)
    .filter(([, value]) => ['string', 'number', 'boolean'].includes(typeof value))
    .map(([key, value]) => ({ key, value: String(value) }))
}

function buildUrlWithQueryParams(requestUrl, queryParams) {
  const baseUrl = getBaseUrl(requestUrl)
  const params = new URLSearchParams()

  for (const param of queryParams) {
    if (param.key) {
      params.append(param.key, param.value || '')
    }
  }

  const queryString = params.toString()
  return queryString ? `${baseUrl}?${queryString}` : baseUrl
}

function getBaseUrl(requestUrl) {
  return splitUrlAndQueryParams(requestUrl).baseUrl
}

function createPathParamRows(baseUrl, previousRows = []) {
  const previousValues = new Map(previousRows.map(row => [row.key, row.value]))
  const matches = Array.from(baseUrl.matchAll(/\{([^}/?]+)\}/g))
  const names = [...new Set(matches.map(match => match[1]))]

  return names.map(key => ({
    key,
    value: previousValues.get(key) || '',
  }))
}

function buildUrlWithPathParams(template, params) {
  return params.reduce((nextUrl, param) => {
    if (!param.value) {
      return nextUrl
    }
    return nextUrl.replaceAll(`{${param.key}}`, encodeURIComponent(param.value))
  }, template)
}

export default RequestBuilder
