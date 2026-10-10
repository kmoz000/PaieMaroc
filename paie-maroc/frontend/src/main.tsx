import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import App from './App'
import {installInspectionGuard} from './inspection-guard'

if (import.meta.env.PROD) {
    installInspectionGuard()
}

const container = document.getElementById('root')

const root = createRoot(container!)

root.render(
    <React.StrictMode>
        <App/>
    </React.StrictMode>
)
