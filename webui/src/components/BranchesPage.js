import React, {useMemo, useEffect, useRef, useState} from "react";

import ButtonToolbar from "react-bootstrap/ButtonToolbar";
import {connect} from "react-redux";
import {listBranches, listBranchesPaginate, createBranch, resetBranch, deleteBranch} from "../actions/branches";
import Modal from "react-bootstrap/Modal";
import Form from "react-bootstrap/Form";
import Button from "react-bootstrap/Button";
import {GitBranchIcon, LinkIcon, LinkExternalIcon, BrowserIcon, TrashcanIcon, PlayIcon} from "@primer/octicons-react";
import Alert from "react-bootstrap/Alert";
import ListGroup from "react-bootstrap/ListGroup";
import ListGroupItem from "react-bootstrap/ListGroupItem";
import Badge from "react-bootstrap/Badge";
import ButtonGroup from "react-bootstrap/ButtonGroup";

import ClipboardButton from "./ClipboardButton";
import OverlayTrigger from "react-bootstrap/OverlayTrigger";
import Tooltip from "react-bootstrap/Tooltip";
import RefDropdown from "./RefDropdown";
import ConfirmationModal from "./ConfirmationModal"
import {Link} from "react-router-dom";


const ImportBranchName = 'import-from-inventory';


const CreateBranchButton = connect(
    ({ branches }) => ({ status: branches.create }),
    ({ createBranch, resetBranch })
)(({ repo, status, createBranch, resetBranch }) => {
    const [show, setShow] = useState(false);
    const defaultBranch = useMemo(() => ({ id: repo.default_branch, type: "branch"}), [repo]);
    const [selectedBranch, setSelectedBranch] = useState(defaultBranch);
    const textRef = useRef(null);

    const disabled = (status.inProgress);

    const onHide = () => {
        if (disabled) return;
        setShow(false);
        setSelectedBranch(defaultBranch);
    };

    const onSubmit = () => {
        if (disabled) return;
        createBranch(repo.id, textRef.current.value, (!!selectedBranch) ? selectedBranch.id : "");
    };

    useEffect(() => {
        if (status.done) {
            setShow(false);
            setSelectedBranch(defaultBranch);
            resetBranch();
        }
    }, [resetBranch, status.done, defaultBranch]);

    return (
        <>
            <Modal enforceFocus={false} show={show} onHide={onHide}>
                <Modal.Header closeButton>
                    <Modal.Title>Create a New Branch</Modal.Title>
                </Modal.Header>
                <Modal.Body>
                    <Form onSubmit={(e) => {
                        onSubmit();
                        e.preventDefault();
                    }}>
                        <Form.Group controlId="name">
                            <Form.Control type="text" placeholder="Branch Name" name="text" ref={textRef}/>
                        </Form.Group>
                        <Form.Group controlId="source">
                            <RefDropdown
                                repo={repo}
                                emptyText={'Select Source Branch'}
                                prefix={'From '}
                                selected={selectedBranch}
                                selectRef={(refId) => {
                                    setSelectedBranch(refId);
                                }}
                                withCommits={true}
                                withWorkspace={false}/>
                        </Form.Group>
                    </Form>
                    {(!!status.error) ? (<Alert variant="danger">{status.error}</Alert>) : (<span/>)}
                </Modal.Body>
                <Modal.Footer>
                    <Button variant="secondary" disabled={disabled} onClick={onHide}>
                        Cancel
                    </Button>
                    <Button variant="success" disabled={disabled} onClick={onSubmit}>
                        Create Branch
                    </Button>
                </Modal.Footer>
            </Modal>
            <Button variant="success" onClick={() => { setShow(true) }}>
                <GitBranchIcon/> Create New Branch
            </Button>
        </>
    );
});

const BranchesPage = connect(
    ({ branches }) => ({ deleteStatus: branches.delete }),
    ({ deleteBranch })
)(({repo, branches, listBranches, listBranchesPaginate, createStatus, deleteBranch, deleteStatus }) => {

    const buttonVariant = "secondary";
    const [selectedBranch, setSelectedBranch] = useState("");
    const [show,setShow] = useState(false);

    const inProgress = deleteStatus.inProgress;

    const handleClose = () => {
        setShow(false);
    }
    const handleShow = () => setShow(true);

    const onSubmit = () => {
        if (inProgress) return;
        deleteBranch(repo.id, selectedBranch);
        handleClose();
    }

    useEffect(() => {
        listBranches(repo.id, "");
    },[listBranches, repo.id]);

    useEffect(() => {
        if (createStatus.done)
            listBranches(repo.id, "");
    }, [listBranches, createStatus.done, repo.id]);

    useEffect(() => {
        if (deleteStatus.done){
            listBranches(repo.id, "");
            }
    }, [listBranches, deleteStatus.done, repo.id]);


    let body;
    if (branches.loading) {
        body = (<Alert variant="info">Loading</Alert>);
    } else if (!!branches.error) {
        body = (<Alert variant="danger">{branches.error}</Alert> );
    } else {
        let deleteBranchConfirmMsg = <>Are you sure you wish to delete branch <strong>{selectedBranch}</strong> ?</>;
        if (selectedBranch === ImportBranchName) {
            deleteBranchConfirmMsg = <>{deleteBranchConfirmMsg}<br/><Badge variant="warning">Warning</Badge> this is a system branch used for importing data to lakeFS</>;
        }
        body = (
            <>
                <ListGroup className="branches-list pagination-group">
                    {branches.payload.results.map((branch, i) => (
                        <ListGroupItem key={i}>
                            <div className="clearfix">
                                <div className="float-left">
                                    <h6>
                                        <Link to={`/repositories/${repo.id}/tree?branch=${branch.id}`}>{branch.id}</Link>
                                        {' '}
                                        {(repo.default_branch === branch.id) ? (<Badge variant="info">Default</Badge>) : (<span/>)}
                                    </h6>
                                </div>
                                <div className="float-right">
                                    <ButtonGroup className="branch-actions">
                                        {(repo.default_branch !== branch.id) ?
                                        (<OverlayTrigger placement="bottom" overlay={<Tooltip>delete the branch</Tooltip>}>
                                            <Button variant={buttonVariant} 
                                                onClick={() =>{
                                                    setSelectedBranch(branch.id);
                                                    handleShow();
                                                    }
                                            }>
                                            <TrashcanIcon/>
                                            </Button> 
                                        </OverlayTrigger>) : 
                                        (<span/>)}
                                        <ClipboardButton variant={buttonVariant} text={`s3://${repo.id}/${branch.id}/`} tooltip="copy S3 URI to clipboard" icon={<LinkExternalIcon/>}/>
                                        <ClipboardButton variant={buttonVariant} text={`lakefs://${repo.id}@${branch.id}`} tooltip="copy URI to clipboard" icon={<LinkIcon/>}/>
                                        <ClipboardButton variant={buttonVariant} text={branch.id} tooltip="copy ID to clipboard"/>
                                        <OverlayTrigger placement="bottom" overlay={<Tooltip>View branch runs</Tooltip>}>
                                            <Button variant={buttonVariant} as={Link} to={`/repositories/${repo.id}/actions?branch=${branch.id}`} >
                                                <PlayIcon/>
                                            </Button>
                                        </OverlayTrigger>
                                        <OverlayTrigger placement="bottom" overlay={<Tooltip>Explore objects</Tooltip>}>
                                            <Button href={`/repositories/${repo.id}/tree?branch=${branch.id}`} variant={buttonVariant}>
                                                <BrowserIcon/>
                                            </Button>
                                        </OverlayTrigger>
                                        <OverlayTrigger placement="bottom" overlay={<Tooltip>Explore objects at last commit</Tooltip>}>
                                            <Button href={`/repositories/${repo.id}/tree?commit=${branch.commit_id}`} variant={buttonVariant}>
                                                {branch.commit_id.substr(0, 16)}
                                            </Button>
                                        </OverlayTrigger>
                                    </ButtonGroup>
                                    <ConfirmationModal show={show} onHide={handleClose} msg={deleteBranchConfirmMsg} onConfirm={onSubmit}/>
                                </div>
                            </div>
                        </ListGroupItem>
                    ))}
                </ListGroup>
                {(branches.payload.pagination.has_more) ? (
                    <p className="tree-paginator">
                        <Button variant="outline-primary" onClick={() => {
                            listBranchesPaginate(repo.id, branches.payload.pagination.next_offset)
                        }}>Load More</Button>
                    </p>
                ) : (<span/>)}
            </>
        );
    }

    return (
        <div className="mt-3 mb-5">
            <div className="action-bar">
                <ButtonToolbar className="float-right mb-2">
                    <CreateBranchButton repo={repo}/>
                </ButtonToolbar>
            </div>

            {body}
        </div>
    );
});

export default connect(
    ({ branches }) => ({ branches: branches.list, createStatus: branches.create }),
    ({ listBranches, listBranchesPaginate })
)(BranchesPage);
