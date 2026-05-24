import { StrictMode, Component } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.jsx'

class ErrorBoundary extends Component {
  constructor(props) {
    super(props)
    this.state = { error: null }
  }
  static getDerivedStateFromError(error) {
    return { error: error.message }
  }
  render() {
    if (this.state.error) {
      return (
        <div style={{ background: '#0A0A0F', color: '#6366f1', fontFamily: 'monospace', padding: '2rem' }}>
          <div style={{ marginBottom: '1rem', color: '#ffffff40', fontSize: '12px' }}>EIDOLON ERROR</div>
          <pre style={{ color: '#f87171', fontSize: '12px' }}>{this.state.error}</pre>
          <button
            onClick={() => this.setState({ error: null })}
            style={{ marginTop: '1rem', background: '#6366f1', color: 'white', border: 'none', padding: '8px 16px', borderRadius: '6px', cursor: 'pointer', fontSize: '12px' }}
          >
            retry
          </button>
        </div>
      )
    }
    return this.props.children
  }
}

createRoot(document.getElementById('root')).render(
  <StrictMode>
    <ErrorBoundary>
      <App />
    </ErrorBoundary>
  </StrictMode>,
)
