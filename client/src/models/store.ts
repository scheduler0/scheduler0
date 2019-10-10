import {CredentialState, ICredentialModel} from "./credential";

const Store = {
    credentialState: new CredentialState()
};

export interface IStore {
    credentialState: ICredentialModel
}

export default Store;