// @ts-ignore
import React from 'react'
// @ts-ignore
import ReactDOM from 'react-dom'
import CreateStore from "../redux/store";
import {Provider} from "react-redux";

declare var window: {
    __INITIAL_DATA__: any,
};

export const clientRender = (Component: React.ElementType) => {
    console.log(window.__INITIAL_DATA__);

    return ReactDOM.hydrate(
        <Provider store={CreateStore(window.__INITIAL_DATA__)}>
            <Component/>
        </Provider>, document.getElementById('root'));
};