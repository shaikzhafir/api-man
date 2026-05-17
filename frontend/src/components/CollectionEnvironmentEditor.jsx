import { useState, useEffect, useCallback } from 'react'

function getEmptyEnv() {
  return {
    baseURL: '',
    headers: {},
    cookies: {},
    auth: {},
    variables: {},
  }
}

function CollectionEnvironmentEditor({ collection, collectionEnvs, globalEnvironments, selectedEnv, onSave, onClose }) {
  const [editingEnv, setEditingEnv] = useState(selectedEnv)
  const [formData, setFormData] = useState(getEmptyEnv())
  const [isDirty, setIsDirty] = useState(false)
  const [saveStatus, setSaveStatus] = useState(null)

  const loadEnvData = useCallback((envName) => {
    const env = collectionEnvs?.environments?.[envName]
    if (env) {
      setFormData({
        baseURL: env.baseURL || '',
        headers: env.headers || {},
        cookies: env.cookies || {},
        auth: env.auth || {},
        variables: env.variables || {},
      })
    } else {
      setFormData(getEmptyEnv())
    }
    setIsDirty(false)
  }, [collectionEnvs])

  useEffect(() => {
    loadEnvData(editingEnv)
  }, [editingEnv, loadEnvData])

  const envNames = globalEnvironments.map(e => e.name)
  if (envNames.length === 0) envNames.push('dev', 'prod')

  const hasOverride = collectionEnvs?.environments?.[editingEnv] != null

  const handleFieldChange = (field, value) => {
    setFormData(prev => ({ ...prev, [field]: value }))
    setIsDirty(true)
    setSaveStatus(null)
  }

  const handleSave = () => {
    const updated = {
      environments: {
        ...(collectionEnvs?.environments || {}),
        [editingEnv]: {
          baseURL: formData.baseURL,
          headers: formData.headers,
          cookies: formData.cookies,
          auth: formData.auth,
          variables: formData.variables,
        },
      },
    }
    onSave(updated)
    setIsDirty(false)
    setSaveStatus('saved')
    setTimeout(() => setSaveStatus(null), 2000)
  }

  const handleRemoveOverride = () => {
    const updated = {
      environments: { ...(collectionEnvs?.environments || {}) },
    }
    delete updated.environments[editingEnv]
    onSave(updated)
    setFormData(getEmptyEnv())
    setIsDirty(false)
  }

  return (
    <div className="collection-env-editor">
      <div className="env-editor-header">
        <div className="env-editor-title">
          <h3>Environment Config</h3>
          <span className="env-editor-collection">{collection}</span>
        </div>
        <button className="env-editor-close" onClick={onClose}>x</button>
      </div>

      <div className="env-editor-tabs">
        {envNames.map(name => (
          <button
            key={name}
            className={`env-editor-tab ${editingEnv === name ? 'active' : ''} ${collectionEnvs?.environments?.[name] ? 'has-override' : ''}`}
            onClick={() => setEditingEnv(name)}
          >
            {name}
            {collectionEnvs?.environments?.[name] && (
              <span
                className="override-dot"
                aria-label="collection override"
                title="Collection override active"
              />
            )}
          </button>
        ))}
      </div>

      <div className="env-editor-body">
        <div className="env-editor-status-row">
          {hasOverride ? (
            <span className="env-override-badge active">Collection override active</span>
          ) : (
            <span className="env-override-badge">Using global environment</span>
          )}
        </div>

        <div className="env-editor-field">
          <label>Base URL</label>
          <input
            type="text"
            value={formData.baseURL}
            onChange={(e) => handleFieldChange('baseURL', e.target.value)}
            placeholder="https://api.example.com"
            className="env-editor-input"
          />
        </div>

        <KeyValueEditor
          label="Headers"
          data={formData.headers}
          onChange={(val) => handleFieldChange('headers', val)}
          keyPlaceholder="Header name"
          valuePlaceholder="Header value"
        />

        <KeyValueEditor
          label="Cookies"
          data={formData.cookies}
          onChange={(val) => handleFieldChange('cookies', val)}
          keyPlaceholder="Cookie name"
          valuePlaceholder="Cookie value"
        />

        <div className="env-editor-field">
          <label>Auth</label>
          <AuthEditor
            auth={formData.auth}
            onChange={(val) => handleFieldChange('auth', val)}
          />
        </div>

        <KeyValueEditor
          label="Variables"
          data={formData.variables}
          onChange={(val) => handleFieldChange('variables', val)}
          keyPlaceholder="Variable name"
          valuePlaceholder="Value"
        />

        <div className="env-editor-actions">
          <button
            className="send-button"
            onClick={handleSave}
            disabled={!isDirty && hasOverride}
          >
            {saveStatus === 'saved' ? 'Saved' : 'Save'}
          </button>
          {hasOverride && (
            <button className="remove-override-button" onClick={handleRemoveOverride}>
              Remove Override
            </button>
          )}
        </div>
      </div>
    </div>
  )
}

function KeyValueEditor({ label, data, onChange, keyPlaceholder, valuePlaceholder }) {
  const entries = Object.entries(data || {})
  const [rows, setRows] = useState(
    entries.length > 0
      ? entries.map(([k, v]) => ({ key: k, value: v }))
      : [{ key: '', value: '' }]
  )

  useEffect(() => {
    const entries = Object.entries(data || {})
    setRows(
      entries.length > 0
        ? entries.map(([k, v]) => ({ key: k, value: v }))
        : [{ key: '', value: '' }]
    )
  }, [data])

  const emitChange = (newRows) => {
    const obj = {}
    for (const row of newRows) {
      if (row.key) obj[row.key] = row.value
    }
    onChange(obj)
  }

  const updateRow = (index, field, value) => {
    const newRows = rows.map((r, i) => i === index ? { ...r, [field]: value } : r)
    setRows(newRows)
    emitChange(newRows)
  }

  const addRow = () => {
    const newRows = [...rows, { key: '', value: '' }]
    setRows(newRows)
  }

  const removeRow = (index) => {
    const newRows = rows.filter((_, i) => i !== index)
    const result = newRows.length > 0 ? newRows : [{ key: '', value: '' }]
    setRows(result)
    emitChange(result)
  }

  return (
    <div className="env-editor-field">
      <label>{label}</label>
      {rows.map((row, i) => (
        <div key={i} className="header-row">
          <input
            type="text"
            className="header-input"
            placeholder={keyPlaceholder}
            value={row.key}
            onChange={(e) => updateRow(i, 'key', e.target.value)}
          />
          <input
            type="text"
            className="header-input"
            placeholder={valuePlaceholder}
            value={row.value}
            onChange={(e) => updateRow(i, 'value', e.target.value)}
          />
          <button className="remove-button" onClick={() => removeRow(i)}>x</button>
        </div>
      ))}
      <button className="add-button" onClick={addRow}>+ Add</button>
    </div>
  )
}

function AuthEditor({ auth, onChange }) {
  const authType = auth?.type || ''

  const handleTypeChange = (type) => {
    if (type === '') {
      onChange({})
    } else if (type === 'bearer') {
      onChange({ type: 'bearer', token: auth?.token || '' })
    } else if (type === 'basic') {
      onChange({ type: 'basic', username: auth?.username || '', password: auth?.password || '' })
    } else if (type === 'api-key') {
      onChange({ type: 'api-key', key: auth?.key || '', header: auth?.header || 'X-API-Key' })
    }
  }

  return (
    <div className="auth-editor">
      <select
        value={authType}
        onChange={(e) => handleTypeChange(e.target.value)}
        className="env-editor-input"
      >
        <option value="">No Auth</option>
        <option value="bearer">Bearer Token</option>
        <option value="basic">Basic Auth</option>
        <option value="api-key">API Key</option>
      </select>

      {authType === 'bearer' && (
        <input
          type="text"
          className="env-editor-input"
          placeholder="Bearer token"
          value={auth?.token || ''}
          onChange={(e) => onChange({ ...auth, token: e.target.value })}
          style={{ marginTop: '0.4rem' }}
        />
      )}

      {authType === 'basic' && (
        <div className="auth-fields">
          <input
            type="text"
            className="env-editor-input"
            placeholder="Username"
            value={auth?.username || ''}
            onChange={(e) => onChange({ ...auth, username: e.target.value })}
          />
          <input
            type="password"
            className="env-editor-input"
            placeholder="Password"
            value={auth?.password || ''}
            onChange={(e) => onChange({ ...auth, password: e.target.value })}
          />
        </div>
      )}

      {authType === 'api-key' && (
        <div className="auth-fields">
          <input
            type="text"
            className="env-editor-input"
            placeholder="Header name (e.g. X-API-Key)"
            value={auth?.header || ''}
            onChange={(e) => onChange({ ...auth, header: e.target.value })}
          />
          <input
            type="text"
            className="env-editor-input"
            placeholder="API key value"
            value={auth?.key || ''}
            onChange={(e) => onChange({ ...auth, key: e.target.value })}
          />
        </div>
      )}
    </div>
  )
}

export default CollectionEnvironmentEditor
