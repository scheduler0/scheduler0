import * as React from 'react';
import * as ReactDOMServer from 'react-dom/server';
import App from '../app';
import { ServerStyleSheets, ThemeProvider } from '@material-ui/styles';
import theme from '../theme';
import { Provider } from "react-redux";
import CreateStore from "../redux/store";
import { StaticRouter } from "react-router-dom";

import {setJobs} from '../redux/jobs'
import {setCredentials} from '../redux/credential';
import {setProjects} from "../redux/projects";


const htmlString = (body, css, data) => `
<!DOCTYPE html>
<html lang="en">
    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
        <meta name="description" content="Cron server frontend">    
        <link rel="stylesheet" href="https://fonts.googleapis.com/css?family=Roboto:300,400,500,700&display=swap" />  
        <link rel="stylesheet" href="https://fonts.googleapis.com/icon?family=Material+Icons" />
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

export const serverRender = (initialData, url) => {
    const sheets = new ServerStyleSheets();
    const store = CreateStore({});

    store.dispatch(setCredentials(initialData.credentials));
    store.dispatch(setProjects(initialData.projects));
    store.dispatch(setJobs(initialData.jobs));

    const html = ReactDOMServer.renderToString(
        sheets.collect(
            <ThemeProvider theme={theme}>
                <Provider store={store}>
                    <StaticRouter location={url} context={{}}>
                        <App />
                    </StaticRouter>
                </Provider>
            </ThemeProvider>
        )
    );

    const css = sheets.toString();

    return htmlString(html, css, store.getState())
};
