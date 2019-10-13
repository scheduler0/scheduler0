// @ts-ignore
import React from 'react'
// @ts-ignore
import ReactDOM from 'react-dom'
import getStore from "../redux/store";
import {Provider} from "react-redux";

declare var window: {
    __INITIAL_DATA__: any,
};

export const clientRender = (Component: React.ElementType) => ReactDOM.hydrate(
    <Provider store={getStore(window.__INITIAL_DATA__)}>
        <Component />
    </Provider>, document.getElementById('root'));