function EnvironmentSelector({ environments, selected, onChange, envSource, activeBaseURL }) {
  return (
    <div className="environment-selector">
      <div className="env-selector-row">
        <label htmlFor="env-select">Env:</label>
        <select
          id="env-select"
          value={selected}
          onChange={(e) => onChange(e.target.value)}
          className="env-select"
        >
          {environments.map((env) => (
            <option key={env.name} value={env.name}>
              {env.name}
            </option>
          ))}
          {environments.length === 0 && (
            <option value="dev">dev</option>
          )}
        </select>
        {envSource && (
          <span className={`env-source-badge ${envSource}`}>
            {envSource === 'collection' ? 'collection' : 'global'}
          </span>
        )}
      </div>
      {activeBaseURL && (
        <span className="env-base-url">{activeBaseURL}</span>
      )}
    </div>
  )
}

export default EnvironmentSelector
