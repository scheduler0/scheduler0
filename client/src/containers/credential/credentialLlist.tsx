import React from "react";
import { makeStyles } from "@material-ui/core/styles";
import theme from '../../theme';
import {ICredential} from "../../redux/credential";
import CredentialListItem from "./creadentialListItem";

interface IProps {
    credentials: ICredential[]
    onDelete: (id) => Promise<void>
    setCurrentCredentialId: (id: string) => () => void
}

const useStyles = makeStyles(theme => ({
    root: {
        padding: theme.spacing(3, 2),
    },
    container: {
        marginTop: '10px',
        padding: '20px',
    },
    header: {
        height: '50px',
        display: 'flex',
        flexDirection: 'row',
        justifyContent: 'space-between',
        alignItems: 'center'
    }
}));

function Credential(props: IProps) {
    const classes = useStyles(theme);
    const { credentials, onDelete, setCurrentCredentialId } = props;

    return (
        <>
            <div className={classes.container}>
                {credentials && credentials.map((credential) => (
                    <CredentialListItem
                        key={credential.api_key}
                        credential={credential}
                        onDelete={onDelete}
                        setCurrentCredentialId={setCurrentCredentialId(credential.id)}
                    />
                ))}
            </div>
        </>
    );
}

export default Credential;