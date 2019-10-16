// @ts-ignore
import React from 'react'
// @ts-ignore
import ReactDOM from 'react-dom'
import CreateStore from "../redux/store";
import {Provider} from "react-redux";
import {SnackbarProvider} from "notistack";

declare var window: {
    __INITIAL_DATA__: any,
};

export const clientRender = (Component: React.ElementType) => {
    return ReactDOM.hydrate(
        <Provider store={CreateStore(window.__INITIAL_DATA__)}>
            <SnackbarProvider maxSnack={5}>
                <Component/>
            </SnackbarProvider>
        </Provider>, document.getElementById('root'));
};