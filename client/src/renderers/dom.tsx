import React from 'react'
import ReactDOM from 'react-dom'

export const clientRender = (Component: React.ElementType) => 
    ReactDOM.hydrate(<Component />, document.getElementById('root'));
