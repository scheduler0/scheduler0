import React, {useCallback, useState} from "react";
import {ICredential} from "../../redux/credential";
import IconButton from "@material-ui/core/IconButton";
import Tooltip from "@material-ui/core/Tooltip"
import DeleteIcon from '@material-ui/icons/Delete';
import FileCopyIcon from '@material-ui/icons/FileCopy';
import EditIcon from "@material-ui/icons/Edit";
import {CopyToClipboard} from 'react-copy-to-clipboard';
import TableRow from "@material-ui/core/TableRow";
import TableCell from "@material-ui/core/TableCell";


interface IProps {
    credential: ICredential
    onDelete: (id) => void
    setCurrentCredentialId: () => void
}

const CredentialListItem = (props: IProps) => {
    const [copied, setCopied] = useState(false);
    const { credential, onDelete, setCurrentCredentialId } = props;
    const key = credential.api_key.substr(0, 55);

    const handleCopy = useCallback(() => {
        setCopied((copied) => !copied);
        setTimeout(() => {
            setCopied((copied) => !copied);
        }, 1000);
    }, []);

    return (
        <TableRow>
            <TableCell>
                {credential.api_key}
            </TableCell>
            <TableCell align="right">
                {credential.http_referrer_restriction}
            </TableCell>
            <TableCell align="right">
                <Tooltip title="Edit">
                    <IconButton onClick={setCurrentCredentialId}>
                        <EditIcon/>
                    </IconButton>
                </Tooltip>
            </TableCell>
            <TableCell align="right">
                <Tooltip title={copied ? "Copied" : "Copy"}>
                    <IconButton>
                        <CopyToClipboard
                            text={credential.api_key}
                            onCopy={handleCopy}>
                            <FileCopyIcon />
                        </CopyToClipboard>
                    </IconButton>
                </Tooltip>
            </TableCell>
            <TableCell align="right">
                <Tooltip title="Delete">
                    <IconButton onClick={() => onDelete(credential.uuid)}>
                        <DeleteIcon />
                    </IconButton>
                </Tooltip>
            </TableCell>
        </TableRow>
    );
};

export default CredentialListItem;
