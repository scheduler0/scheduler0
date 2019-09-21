import React from 'react'
import ReactDOMServer from 'react-dom/server'
import App from '../app'

const htmlString = (body, data) => `
<!DOCTYPE html>
<html lang="en">
    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
        <meta name="description" content="Cron server frontend">            
        <title>Cron</title>
    </head>
    <body>
        <noscript>
            You need to enable JavaScript to run this app.
        </noscript>
        <div id="root">${body}</div>
        <script type="text/javascript">
            window.__DATA__ = ${JSON.stringify(data)}
        </script>
        <script type="text/javascript" src="public/dist/bundle.js"></script>
    </body>
</html>
`;

export const serverRender = (initialData) => htmlString(ReactDOMServer.renderToString(<App />), initialData);
