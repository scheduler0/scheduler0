import * as React from 'react';
import * as ReactDOMServer from 'react-dom/server';
import App from '../app';
import { ServerStyleSheets, ThemeProvider } from '@material-ui/styles';
import theme from '../theme';
import { Provider } from "react-redux";
import getStore from "../redux/store";
import { CredentialActions } from '../redux/credential'

const htmlString = (body, css, data) => `
<!DOCTYPE html>
<html lang="en">
    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
        <meta name="description" content="Cron server frontend">    
        <link rel="stylesheet" href="https://fonts.googleapis.com/css?family=Roboto:300,400,500,700&display=swap" />  
        <link rel="stylesheet" href="https://fonts.googleapis.com/icon?family=Material+Icons" />
        <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/normalize.css@8.0.1/normalize.css" />
        <style id="jss-server-side">${css}</style>
        <title>Cron Server</title>
    </head>
    <body>
        <noscript>
            You need to enable JavaScript to run this app.
        </noscript>
        <div id="root">${body}</div>
        <script type="text/javascript">
            window.__INITIAL_DATA__ = ${JSON.stringify(data)}
        </script>
        <script type="text/javascript" src="public/dist/bundle.js"></script>
    </body>
</html>
`;

export const serverRender = (initialData) => {
    const sheets = new ServerStyleSheets();
    const store = getStore({});

    store.dispatch({
        type: CredentialActions.SET_CREDENTIALS,
        payload: initialData
    });

    const html = ReactDOMServer.renderToString(
        sheets.collect(
            <ThemeProvider theme={theme}>
                <Provider store={store}>
                    <App />
                </Provider>
            </ThemeProvider>
        )
    );

    const css = sheets.toString();

    return htmlString(html, css, store.getState())
};
