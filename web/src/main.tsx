import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { AppTheme } from './ui/AppTheme/AppTheme'
import controlStyles from './ui/Controls/Controls.module.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <div className={controlStyles.surface}>
      <AppTheme>
        <App />
      </AppTheme>
    </div>
  </StrictMode>,
)
