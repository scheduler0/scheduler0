// @ts-ignore
import express from 'express';
import axiosInstance from '../misc/axiosInstance'

const router = express.Router();

router.route("/")
    .post((req, res) => {
        console.log(req.body);
        axiosInstance
            .post('/credentials', req.body)
            .then((resp) => {
                res.status(resp.status).send(resp.data);
            }).catch((e) => {
                res.status(e.response.status).send(e.response.data);
            });
    })
    .get(async (_, res) => {
        let resp;

        try {
            resp = await axiosInstance.get('/credentials');
            res.status(resp.status).send(resp.data);
        } catch (e) {
            res.status(e.status).send(e.data);
        }
    });

router.route("/:id")
    .delete(async (req, res) => {

        axiosInstance.delete('/credentials/' + req.params.id)
            .then((resp) => {
                res.status(resp.status).send(resp.data);
            })
            .catch((reason) => {
                res.status(400).send(reason.message);
            });
    });

export default router;