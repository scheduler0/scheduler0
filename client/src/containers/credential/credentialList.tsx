import React, {useCallback} from "react";
import { makeStyles } from "@material-ui/core/styles";
import theme from '../../theme';
import {ICredential} from "../../redux/credential";
import CredentialListItem from "./creadentialListItem";
import Box from "@material-ui/core/Box";
import Button from "@material-ui/core/Button";
import {FormMode} from "./index";
import Table from "@material-ui/core/Table";
import TableHead from "@material-ui/core/TableHead";
import TableRow from "@material-ui/core/TableRow";
import TableCell from "@material-ui/core/TableCell";
import TableBody from "@material-ui/core/TableBody";
import TableContainer from "@material-ui/core/TableContainer";

interface IProps {
    credentials: ICredential[]
    setCurrentCredentialId: (id: string) => void
    formMode: FormMode
    deleteCredential: (id: string) => Promise<void>
    setMode: (mode: FormMode) => void
}

const useStyles = makeStyles(theme => ({
    root: {},
    container: {
        maxHeight: 'calc(100vh - 180px)',
    },
    header: {
        marginTop: '50px',
        height: '50px',
        display: 'flex',
        flexDirection: 'row',
        justifyContent: 'space-between',
        alignItems: 'center'
    }
}));

function CredentialList(props: IProps) {
    const classes = useStyles(theme);
    const { credentials, deleteCredential, setCurrentCredentialId, setMode } = props;

    const handleDelete = useCallback((credentialId) => {
        deleteCredential(credentialId);
    }, []);

    return (
        <div className={classes.container}>
            <Box display="flex"
                 component="div"
                 flexDirection="row"
                 alignItems="center"
                 justifyContent="flex-end"
                 style={{ margin: '25px 0px', padding: '0px 20px' }}>
                <Button component="span" onClick={() => {
                    setCurrentCredentialId(null);
                    setMode(FormMode.Create);
                }}>Create New Key</Button>
            </Box>
            <TableContainer className={classes.container}>
                <Table stickyHeader>
                    <TableHead>
                        <TableRow>
                            <TableCell>API Key</TableCell>
                            <TableCell align="right">HTTP Referral Restriction</TableCell>
                            <TableCell align="right"/>
                            <TableCell align="right"/>
                            <TableCell align="right"/>
                        </TableRow>
                    </TableHead>
                    <TableBody>
                        {credentials.map((credential, index) => (
                            <CredentialListItem
                                key={`credential-${index}`}
                                credential={credential}
                                onDelete={handleDelete}
                                setCurrentCredentialId={() => {
                                    setCurrentCredentialId(credential.uuid);
                                    setMode(FormMode.Edit);
                                }}
                            />
                        ))}
                    </TableBody>
                </Table>
            </TableContainer>
        </div>
    );
}

export default CredentialList;
