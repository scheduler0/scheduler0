import App from './app'
import { clientRender } from './renderers/dom'

if (window) {
    window.onload = () => clientRender(App)
}

if (module.hot) {
    module.hot.accept("./app", () => {
        const NewApp = require("./app").default;
        clientRender(NewApp)
    });
}