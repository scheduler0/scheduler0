// @ts-ignore
import React from 'react'
// @ts-ignore
import ReactDOM from 'react-dom'
import CreateStore from "../redux/store";
import {Provider} from "react-redux";
import {BrowserRouter} from "react-router-dom";
import {SnackbarProvider} from "notistack";

declare var window: {
    __INITIAL_DATA__: any,
};

export const clientRender = (Component: React.ElementType) => {
    return ReactDOM.hydrate(
        <Provider store={CreateStore(window.__INITIAL_DATA__)}>
            <SnackbarProvider maxSnack={5}>
                <BrowserRouter>
                    <Component/>
                </BrowserRouter>
            </SnackbarProvider>
        </Provider>, document.getElementById('root'));
};