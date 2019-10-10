import React from 'react'
import ReactDOM from 'react-dom'
import {CredentialState} from "../models/credential";
import Store from "../models/store";
import {Provider} from "mobx-react";

let store = new CredentialState();

declare var window: {
    __INITIAL_DATA__: typeof Store,
};

if (typeof window != 'undefined') {
    store.setCredentials([]);
}

export const clientRender = (Component: React.ElementType) => 
    ReactDOM.hydrate(<Component rootStore={store} />, document.getElementById('root'));
