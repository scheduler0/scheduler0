import {combineReducers, createStore, applyMiddleware, compose} from "redux";
import reduxThunk from "redux-thunk"
import {credentialsReducer} from "./credential";

declare var window: {
    __REDUX_DEVTOOLS_EXTENSION_COMPOSE__: any,
};

const composeEnhancers = window.__REDUX_DEVTOOLS_EXTENSION_COMPOSE__ || compose;

const reducers = combineReducers({
    CredentialsReducer: credentialsReducer
});

const getStore = (preloadState = null) => createStore(reducers, preloadState, composeEnhancers(applyMiddleware(reduxThunk)));

export default getStore;