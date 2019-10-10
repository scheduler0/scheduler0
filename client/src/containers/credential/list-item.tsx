import React, {useCallback, useState} from "react";
import {ICredential} from "../../models/credential";
import Divider from "@material-ui/core/Divider";
import ListItemText from "@material-ui/core/ListItemText";
import IconButton from "@material-ui/core/IconButton";
import Tooltip from "@material-ui/core/Tooltip"
import DeleteIcon from '@material-ui/icons/Delete';
import FileCopyIcon from '@material-ui/icons/FileCopy';
import EditIcon from "@material-ui/icons/Edit";
import {CopyToClipboard} from 'react-copy-to-clipboard';
import {ListItem} from "@material-ui/core";


interface IProps {
    credential: ICredential
    onDelete: (id) => Promise<void>
}

const CredentialListItem = (props: IProps) => {
    const [copied, setCopied] = useState(false);
    const { credential, onDelete } = props;
    const key = credential.api_key.substr(0, 55);

    const handleCopy = useCallback(() => {
        setCopied((copied) => !copied);
        setTimeout(() => {
            setCopied((copied) => !copied);
        }, 1000);
    }, []);

    return (
        <div key={key}>
            <Divider />
            <ListItem>
                <ListItemText
                    primary={key}
                    secondary={credential.http_referrer_restriction}
                />
                <Tooltip title="Edit">
                    <IconButton>
                        <EditIcon />
                    </IconButton>
                </Tooltip>
                <Tooltip title={copied ? "Copied" : "Copy"}>
                    <IconButton>
                        <CopyToClipboard
                            text={credential.api_key}
                            onCopy={handleCopy}>
                            <FileCopyIcon />
                        </CopyToClipboard>
                    </IconButton>
                </Tooltip>
                <Tooltip title="Delete">
                    <IconButton onClick={() => onDelete(credential.id)}>
                        <DeleteIcon />
                    </IconButton>
                </Tooltip>
            </ListItem>
        </div>
    );
};

export default CredentialListItem;